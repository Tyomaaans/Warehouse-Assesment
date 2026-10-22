package products

import (
	"time"

	"warehouse/pkg"
)

type CreateProductRequest struct {
	SKU  string `json:"sku" validate:"required,notblank"`
	Name string `json:"name" validate:"required,notblank"`
}

type ProductResponse struct {
	ID        string    `json:"id"`
	SKU       string    `json:"sku"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type PaginatedProductResponse struct {
	Products []ProductResponse  `json:"products"`
	Meta     pkg.PaginationMeta `json:"meta"`
}
