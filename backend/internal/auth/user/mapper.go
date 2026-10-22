package user

import (
	"warehouse/internal/auth/domain"
	"warehouse/pkg"
)

func ToUserEntity(s *UserStorage) *domain.UserEntity {
	return &domain.UserEntity{
		ID: s.ID, Email: s.Email, PasswordHash: s.PasswordHash,
		Role: s.Role, CreatedAt: s.CreatedAt,
	}
}

func ToUserResponse(e domain.UserEntity) UserResponse {
	return UserResponse{ID: pkg.EncodeID(e.ID), Email: e.Email, Role: e.Role}
}
