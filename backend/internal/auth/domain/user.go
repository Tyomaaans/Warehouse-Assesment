package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"warehouse/pkg"
)

type UserEntity struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

type UserRepository interface {
	GetUserByEmail(context.Context, string) (*UserEntity, error)
	GetUserByID(context.Context, uuid.UUID) (*UserEntity, error)
}

func ValidRole(role string) bool {
	return role == pkg.Operator || role == pkg.Supervisor
}
