package user

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"warehouse/pkg"
)

type ProvisionAccount struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
	Role     string `json:"role" validate:"required,oneof=OPERATOR SUPERVISOR"`
}

func ProvisionUsers(ctx context.Context, db *gorm.DB, validate *validator.Validate, accounts []ProvisionAccount) (int, error) {
	storages := make([]UserStorage, len(accounts))
	for i, account := range accounts {
		if err := pkg.ValidateStruct(validate, account); err != nil {
			return 0, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(account.Password), bcrypt.DefaultCost)
		if err != nil {
			return 0, fmt.Errorf("%w: provisioning password must contain at most 72 bytes", pkg.ErrInvalidInput)
		}
		storages[i] = UserStorage{
			ID: uuid.New(), Email: strings.ToLower(account.Email), PasswordHash: string(hash),
			Role: account.Role, CreatedAt: time.Now().UTC(),
		}
	}
	created := 0
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, storage := range storages {
			result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "email"}}, DoNothing: true}).Create(&storage)
			if result.Error != nil {
				return pkg.HandleDBError(result.Error)
			}
			if result.RowsAffected == 0 {
				var existing UserStorage
				if err := tx.Where("email = ?", storage.Email).First(&existing).Error; err != nil {
					return pkg.HandleDBError(err)
				}
				if existing.Role != storage.Role {
					return fmt.Errorf("%w: provisioning email already belongs to a different role", pkg.ErrConflict)
				}
			}
			created += int(result.RowsAffected)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return created, nil
}
