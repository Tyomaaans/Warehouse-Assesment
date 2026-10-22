package token

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"warehouse/internal/auth/domain"
	"warehouse/internal/auth/session"
	"warehouse/pkg"
)

const AccessTokenExpiry = 15 * time.Minute

// Interface
type JWTService interface {
	GenerateTokenPair(context.Context, *domain.UserEntity, bool) (*domain.TokenPairEntity, error)
	ValidateAccessToken(context.Context, string) (pkg.Actor, error)
	RefreshTokens(context.Context, string, uuid.UUID) (*domain.TokenPairEntity, error)
	RevokeTokens(context.Context, pkg.Actor, string, uuid.UUID) error
}

// Implementation
type jwtClaims struct {
	SessionID string `json:"sid"`
	Role      string `json:"role"`
	gojwt.RegisteredClaims
}

type jwtService struct {
	secretKey            []byte
	defaultRefreshExpiry time.Duration
	shortRefreshExpiry   time.Duration
	redisClient          *redis.Client
	sessionSvc           session.SessionService
	userRepo             domain.UserRepository
}

func NewJWTService(secretKey string, defaultRefreshExpiry, shortRefreshExpiry time.Duration, redisClient *redis.Client, sessionSvc session.SessionService, userRepo domain.UserRepository) JWTService {
	return &jwtService{
		secretKey: []byte(secretKey), defaultRefreshExpiry: defaultRefreshExpiry,
		shortRefreshExpiry: shortRefreshExpiry, redisClient: redisClient,
		sessionSvc: sessionSvc, userRepo: userRepo,
	}
}

func (s *jwtService) GenerateTokenPair(ctx context.Context, user *domain.UserEntity, rememberMe bool) (*domain.TokenPairEntity, error) {
	if !validUUID(user.ID.String()) || !domain.ValidRole(user.Role) {
		return nil, pkg.ErrInvalidCredentials
	}
	ttl := s.shortRefreshExpiry
	if rememberMe {
		ttl = s.defaultRefreshExpiry
	}
	sessionID := uuid.NewString()
	raw, hash, err := generateRefreshToken()
	if err != nil {
		return nil, err
	}
	access, err := s.generateAccessToken(user.ID.String(), sessionID, user.Role)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sess := &domain.SessionEntity{
		SessionID: sessionID, UserID: user.ID.String(), Role: user.Role, RefreshToken: hash,
		IssuedAt: now, LastActiveAt: now, ExpiresAt: now.Add(ttl),
	}
	payload := &domain.RefreshTokenEntity{
		UserID: user.ID.String(), SessionID: sessionID, Role: user.Role, RememberMe: rememberMe,
		CreatedAt: now, ExpiresAt: now.Add(ttl), TTL: ttl,
	}
	if err := s.sessionSvc.CreateSession(ctx, sess, payload, ttl); err != nil {
		return nil, err
	}
	return &domain.TokenPairEntity{AccessToken: access, RefreshToken: raw, SessionID: sessionID, TTL: ttl}, nil
}

func (s *jwtService) ValidateAccessToken(ctx context.Context, raw string) (pkg.Actor, error) {
	claims := &jwtClaims{}
	parsed, err := gojwt.ParseWithClaims(raw, claims, func(t *gojwt.Token) (any, error) {
		return s.secretKey, nil
	}, gojwt.WithValidMethods([]string{"HS256"}), gojwt.WithExpirationRequired(), gojwt.WithIssuedAt())
	if err != nil || !parsed.Valid || claims.IssuedAt == nil || claims.ExpiresAt == nil {
		return pkg.Actor{}, pkg.ErrUnauthorized
	}
	if !validUUID(claims.Subject) || !validUUID(claims.SessionID) || !domain.ValidRole(claims.Role) ||
		claims.ExpiresAt.Sub(claims.IssuedAt.Time) != AccessTokenExpiry {
		return pkg.Actor{}, pkg.ErrUnauthorized
	}
	sess, err := s.sessionSvc.GetSession(ctx, claims.SessionID)
	if err != nil {
		return pkg.Actor{}, err
	}
	if sess.UserID != claims.Subject || sess.Role != claims.Role {
		return pkg.Actor{}, pkg.ErrUnauthorized
	}
	userID := uuid.MustParse(claims.Subject)
	if err := s.validateUser(ctx, userID, claims.Role); err != nil {
		return pkg.Actor{}, err
	}
	return pkg.Actor{ID: userID, SessionID: uuid.MustParse(claims.SessionID), Role: claims.Role}, nil
}

func (s *jwtService) RefreshTokens(ctx context.Context, raw string, sessionID uuid.UUID) (*domain.TokenPairEntity, error) {
	if !validRefreshToken(raw) {
		return nil, pkg.ErrUnauthorized
	}
	hash := hashToken(raw)
	lockKey, owner := "lock:refresh:"+hash, uuid.NewString()
	locked, err := s.redisClient.SetNX(ctx, lockKey, owner, 5*time.Second).Result()
	if err != nil {
		return nil, pkg.ErrUnavailable
	}
	if !locked {
		return nil, pkg.ErrConcurrentRefresh
	}
	defer s.releaseLock(lockKey, owner)

	payload, err := s.sessionSvc.GetRefreshToken(ctx, hash)
	if err != nil {
		return nil, err
	}
	if payload.SessionID != sessionID.String() || !validUUID(payload.UserID) || !domain.ValidRole(payload.Role) {
		return nil, pkg.ErrUnauthorized
	}
	sess, err := s.sessionSvc.GetSession(ctx, payload.SessionID)
	if err != nil {
		return nil, err
	}
	if sess.UserID != payload.UserID || sess.Role != payload.Role || sess.RefreshToken != hash {
		return nil, pkg.ErrUnauthorized
	}
	if err := s.validateUser(ctx, uuid.MustParse(payload.UserID), payload.Role); err != nil {
		return nil, err
	}
	rawRefresh, newHash, err := generateRefreshToken()
	if err != nil {
		return nil, err
	}
	access, err := s.generateAccessToken(payload.UserID, payload.SessionID, payload.Role)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sess.RefreshToken, sess.LastActiveAt, sess.ExpiresAt = newHash, now, now.Add(payload.TTL)
	payload.CreatedAt, payload.ExpiresAt = now, sess.ExpiresAt
	if err := s.sessionSvc.RotateSession(ctx, sess, payload, hash, owner); err != nil {
		return nil, err
	}
	return &domain.TokenPairEntity{AccessToken: access, RefreshToken: rawRefresh, SessionID: payload.SessionID, TTL: payload.TTL}, nil
}

func (s *jwtService) RevokeTokens(ctx context.Context, actor pkg.Actor, raw string, sessionID uuid.UUID) error {
	verified, err := pkg.RequireActor(pkg.WithActor(ctx, actor), "")
	if err != nil {
		return err
	}
	if verified.SessionID != sessionID || !validRefreshToken(raw) {
		return pkg.ErrUnauthorized
	}
	hash := hashToken(raw)
	payload, err := s.sessionSvc.GetRefreshToken(ctx, hash)
	if err != nil {
		return err
	}
	if payload.SessionID != sessionID.String() || payload.UserID != verified.ID.String() || payload.Role != verified.Role {
		return pkg.ErrUnauthorized
	}
	return s.sessionSvc.RevokeSession(ctx, verified.ID.String(), sessionID.String(), hash)
}

func (s *jwtService) validateUser(ctx context.Context, id uuid.UUID, role string) error {
	user, err := s.userRepo.GetUserByID(ctx, id)
	if errors.Is(err, pkg.ErrNotFound) {
		return pkg.ErrUnauthorized
	}
	if err != nil {
		return err
	}
	if user.Role != role || !domain.ValidRole(user.Role) {
		return pkg.ErrUnauthorized
	}
	return nil
}

func (s *jwtService) generateAccessToken(userID, sessionID, role string) (string, error) {
	now := time.Now().UTC()
	claims := &jwtClaims{
		SessionID: sessionID, Role: role,
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject: userID, IssuedAt: gojwt.NewNumericDate(now), ExpiresAt: gojwt.NewNumericDate(now.Add(AccessTokenExpiry)),
		},
	}
	signed, err := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims).SignedString(s.secretKey)
	if err != nil {
		return "", pkg.ErrInternal
	}
	return signed, nil
}

func generateRefreshToken() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", pkg.ErrInternal
	}
	raw := hex.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func validRefreshToken(raw string) bool {
	if len(raw) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(raw)
	return err == nil && hex.EncodeToString(decoded) == raw
}

func validUUID(raw string) bool {
	id, err := uuid.Parse(raw)
	return err == nil && id.Version() == 4 && id.Variant() == uuid.RFC4122 && id.String() == raw
}

var releaseLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) end
return 0
`)

func (s *jwtService) releaseLock(key, owner string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = releaseLockScript.Run(ctx, s.redisClient, []string{key}, owner).Err()
}
