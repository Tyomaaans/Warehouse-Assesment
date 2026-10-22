package user

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"warehouse/internal/auth/domain"
	"warehouse/pkg"
)

type userRepositoryStub struct {
	user *domain.UserEntity
	err  error
}

func (r *userRepositoryStub) GetUserByEmail(context.Context, string) (*domain.UserEntity, error) {
	return r.user, r.err
}

func (r *userRepositoryStub) GetUserByID(context.Context, uuid.UUID) (*domain.UserEntity, error) {
	return r.user, r.err
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	userRepo := &userRepositoryStub{user: &domain.UserEntity{
		ID: uuid.New(), Email: "operator@example.test", PasswordHash: string(hash), Role: pkg.Operator,
	}}
	service := NewUserService(userRepo, nil, pkg.NewValidator())
	_, _, err = service.LoginUser(context.Background(), LoginRequest{Email: "operator@example.test", Password: "wrong-password"})
	if !errors.Is(err, pkg.ErrInvalidCredentials) {
		t.Fatalf("LoginUser() error = %v, want invalid credentials", err)
	}
	userRepo.err = pkg.ErrNotFound
	_, _, err = service.LoginUser(context.Background(), LoginRequest{Email: "missing@example.test", Password: "wrong-password"})
	if !errors.Is(err, pkg.ErrInvalidCredentials) {
		t.Fatalf("missing user error = %v, want invalid credentials", err)
	}
}

func TestLoginPreservesDependencyFailures(t *testing.T) {
	service := NewUserService(&userRepositoryStub{err: pkg.ErrUnavailable}, nil, pkg.NewValidator())
	_, _, err := service.LoginUser(context.Background(), LoginRequest{Email: "operator@example.test", Password: "password"})
	if !errors.Is(err, pkg.ErrUnavailable) {
		t.Fatalf("LoginUser() error = %v, want unavailable", err)
	}
}

func TestLoginRejectsBcryptTruncation(t *testing.T) {
	password := strings.Repeat("a", 72)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repo := &userRepositoryStub{user: &domain.UserEntity{
		ID: uuid.New(), Email: "operator@example.test", PasswordHash: string(hash), Role: pkg.Operator,
	}}
	service := NewUserService(repo, nil, pkg.NewValidator())
	_, _, err = service.LoginUser(context.Background(), LoginRequest{Email: repo.user.Email, Password: password + "extra"})
	if !errors.Is(err, pkg.ErrInvalidCredentials) {
		t.Fatalf("overlength password error = %v, want invalid credentials", err)
	}
}

func TestLogoutRequiresServiceIdentity(t *testing.T) {
	service := NewUserService(nil, nil, pkg.NewValidator())
	err := service.LogoutUser(context.Background(), "token", uuid.New())
	if !errors.Is(err, pkg.ErrUnauthorized) {
		t.Fatalf("LogoutUser() error = %v, want unauthorized", err)
	}
}
