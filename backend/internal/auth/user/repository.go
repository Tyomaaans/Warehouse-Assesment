package user

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"warehouse/internal/auth/domain"
	"warehouse/pkg"
)

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetUserByEmail(ctx context.Context, email string) (*domain.UserEntity, error) {
	var storage UserStorage
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&storage).Error; err != nil {
		return nil, pkg.HandleDBError(err)
	}
	return ToUserEntity(&storage), nil
}

func (r *userRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.UserEntity, error) {
	var storage UserStorage
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&storage).Error; err != nil {
		return nil, pkg.HandleDBError(err)
	}
	return ToUserEntity(&storage), nil
}
