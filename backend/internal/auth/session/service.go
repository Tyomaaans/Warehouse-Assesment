package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"warehouse/internal/auth/domain"
	"warehouse/pkg"
)

// Interface
type SessionService interface {
	CreateSession(context.Context, *domain.SessionEntity, *domain.RefreshTokenEntity, time.Duration) error
	GetSession(context.Context, string) (*domain.SessionEntity, error)
	GetRefreshToken(context.Context, string) (*domain.RefreshTokenEntity, error)
	RotateSession(context.Context, *domain.SessionEntity, *domain.RefreshTokenEntity, string, string) error
	RevokeSession(context.Context, string, string, string) error
}

// Implementation
type sessionService struct {
	redisClient *redis.Client
}

func NewSessionService(redisClient *redis.Client) SessionService {
	return &sessionService{redisClient: redisClient}
}

func HashID(id string) string {
	h := sha256.Sum256([]byte(id))
	return hex.EncodeToString(h[:])
}

func (s *sessionService) CreateSession(ctx context.Context, sess *domain.SessionEntity, payload *domain.RefreshTokenEntity, ttl time.Duration) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return pkg.ErrInternal
	}
	refresh, err := json.Marshal(payload)
	if err != nil {
		return pkg.ErrInternal
	}
	result, err := createSessionScript.Run(ctx, s.redisClient, []string{
		"session:" + HashID(sess.SessionID), "refresh:" + sess.RefreshToken, "user_sessions:" + sess.UserID,
	}, data, refresh, ttl.Milliseconds(), HashID(sess.SessionID)).Int()
	if err != nil {
		return pkg.ErrUnavailable
	}
	if result != 1 {
		return pkg.ErrInternal
	}
	return nil
}

func (s *sessionService) GetSession(ctx context.Context, sessionID string) (*domain.SessionEntity, error) {
	data, err := s.redisClient.Get(ctx, "session:"+HashID(sessionID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, pkg.ErrUnauthorized
	}
	if err != nil {
		return nil, pkg.ErrUnavailable
	}
	var sess domain.SessionEntity
	if json.Unmarshal(data, &sess) != nil {
		return nil, pkg.ErrInternal
	}
	if sess.SessionID != sessionID || !time.Now().Before(sess.ExpiresAt) {
		return nil, pkg.ErrUnauthorized
	}
	return &sess, nil
}

func (s *sessionService) GetRefreshToken(ctx context.Context, hash string) (*domain.RefreshTokenEntity, error) {
	data, err := s.redisClient.Get(ctx, "refresh:"+hash).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, pkg.ErrUnauthorized
	}
	if err != nil {
		return nil, pkg.ErrUnavailable
	}
	var payload domain.RefreshTokenEntity
	if json.Unmarshal(data, &payload) != nil {
		return nil, pkg.ErrInternal
	}
	if !time.Now().Before(payload.ExpiresAt) || payload.TTL < time.Second {
		return nil, pkg.ErrUnauthorized
	}
	return &payload, nil
}

func (s *sessionService) RotateSession(ctx context.Context, sess *domain.SessionEntity, payload *domain.RefreshTokenEntity, oldHash, lockOwner string) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return pkg.ErrInternal
	}
	refresh, err := json.Marshal(payload)
	if err != nil {
		return pkg.ErrInternal
	}
	result, err := rotateSessionScript.Run(ctx, s.redisClient, []string{
		"session:" + HashID(sess.SessionID), "refresh:" + oldHash,
		"refresh:" + sess.RefreshToken, "user_sessions:" + sess.UserID, "lock:refresh:" + oldHash,
	}, sess.UserID, sess.SessionID, oldHash, data, refresh, payload.TTL.Milliseconds(), HashID(sess.SessionID), lockOwner).Int()
	if err != nil {
		return pkg.ErrUnavailable
	}
	if result == -1 {
		return pkg.ErrConcurrentRefresh
	}
	if result == -2 {
		return pkg.ErrInternal
	}
	if result != 1 {
		return pkg.ErrUnauthorized
	}
	return nil
}

func (s *sessionService) RevokeSession(ctx context.Context, userID, sessionID, refreshHash string) error {
	result, err := revokeSessionScript.Run(ctx, s.redisClient, []string{
		"session:" + HashID(sessionID), "user_sessions:" + userID, "refresh:" + refreshHash,
	}, userID, sessionID, HashID(sessionID)).Int()
	if err != nil {
		return pkg.ErrUnavailable
	}
	if result != 1 {
		return pkg.ErrUnauthorized
	}
	return nil
}

var createSessionScript = redis.NewScript(`
if redis.call("EXISTS", KEYS[1], KEYS[2]) > 0 then return 0 end
redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[3])
redis.call("SET", KEYS[2], ARGV[2], "PX", ARGV[3])
redis.call("SADD", KEYS[3], ARGV[4])
if redis.call("PTTL", KEYS[3]) < tonumber(ARGV[3]) then
  redis.call("PEXPIRE", KEYS[3], ARGV[3])
end
return 1
`)

var rotateSessionScript = redis.NewScript(`
if redis.call("GET", KEYS[5]) ~= ARGV[8] then return -1 end
local data = redis.call("GET", KEYS[1])
if not data or redis.call("EXISTS", KEYS[2]) == 0 then return 0 end
local sess = cjson.decode(data)
if sess.UserID ~= ARGV[1] or sess.SessionID ~= ARGV[2] or sess.RefreshToken ~= ARGV[3] then return 0 end
if redis.call("EXISTS", KEYS[3]) == 1 then return -2 end
redis.call("DEL", KEYS[2])
redis.call("SET", KEYS[1], ARGV[4], "PX", ARGV[6])
redis.call("SET", KEYS[3], ARGV[5], "PX", ARGV[6])
redis.call("SADD", KEYS[4], ARGV[7])
if redis.call("PTTL", KEYS[4]) < tonumber(ARGV[6]) then
  redis.call("PEXPIRE", KEYS[4], ARGV[6])
end
return 1
`)

var revokeSessionScript = redis.NewScript(`
local data = redis.call("GET", KEYS[1])
if not data then return 0 end
local sess = cjson.decode(data)
if sess.UserID ~= ARGV[1] or sess.SessionID ~= ARGV[2] then return 0 end
redis.call("DEL", "refresh:" .. sess.RefreshToken, KEYS[1], KEYS[3])
redis.call("SREM", KEYS[2], ARGV[3])
return 1
`)
