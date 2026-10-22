package products

import (
	"time"

	"github.com/google/uuid"
)

type ProductStorage struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	SKU       string
	Name      string
	CreatedAt time.Time
}

func (ProductStorage) TableName() string { return "products" }
