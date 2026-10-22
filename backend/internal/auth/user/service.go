package user

import (
	"context"
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"warehouse/internal/auth/domain"
	"warehouse/internal/auth/token"
	"warehouse/pkg"
)

// Interface
type UserService interface {
	LoginUser(context.Context, LoginRequest) (*LoginResponse, *domain.TokenPairEntity, error)
	RefreshTokens(context.Context, string, uuid.UUID) (*domain.TokenPairEntity, error)
	LogoutUser(context.Context, string, uuid.UUID) error
}

// Implementation
type userService struct {
	userRepo domain.UserRepository
	tokenSvc token.JWTService
	validate *validator.Validate
}

func NewUserService(userRepo domain.UserRepository, tokenSvc token.JWTService, validate *validator.Validate) UserService {
	return &userService{userRepo: userRepo, tokenSvc: tokenSvc, validate: validate}
}

const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func (s *userService) LoginUser(ctx context.Context, req LoginRequest) (*LoginResponse, *domain.TokenPairEntity, error) {
	if err := pkg.ValidateStruct(s.validate, req); err != nil {
		return nil, nil, err
	}
	if len(req.Password) > 72 {
		return nil, nil, pkg.ErrInvalidCredentials
	}
	user, err := s.userRepo.GetUserByEmail(ctx, strings.ToLower(req.Email))
	if errors.Is(err, pkg.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(req.Password))
		return nil, nil, pkg.ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil || !domain.ValidRole(user.Role) {
		return nil, nil, pkg.ErrInvalidCredentials
	}
	pair, err := s.tokenSvc.GenerateTokenPair(ctx, user, bool(req.RememberMe))
	if err != nil {
		return nil, nil, err
	}
	return &LoginResponse{User: ToUserResponse(*user), AccessToken: pair.AccessToken, ExpiresIn: int(token.AccessTokenExpiry.Seconds())}, pair, nil
}

func (s *userService) RefreshTokens(ctx context.Context, raw string, sessionID uuid.UUID) (*domain.TokenPairEntity, error) {
	return s.tokenSvc.RefreshTokens(ctx, raw, sessionID)
}

func (s *userService) LogoutUser(ctx context.Context, raw string, sessionID uuid.UUID) error {
	actor, err := pkg.RequireActor(ctx, "")
	if err != nil {
		return err
	}
	return s.tokenSvc.RevokeTokens(ctx, actor, raw, sessionID)
}
