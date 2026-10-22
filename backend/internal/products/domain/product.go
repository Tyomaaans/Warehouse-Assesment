package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ProductEntity struct {
	ID        uuid.UUID
	SKU       string
	Name      string
	CreatedAt time.Time
}

type ProductRepository interface {
	CreateProduct(context.Context, *ProductEntity) error
	GetProducts(context.Context, string, int, int) ([]ProductEntity, int64, error)
}
