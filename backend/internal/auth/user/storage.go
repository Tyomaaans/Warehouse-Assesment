package user

import (
	"time"

	"github.com/google/uuid"
)

type UserStorage struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

func (UserStorage) TableName() string { return "users" }
