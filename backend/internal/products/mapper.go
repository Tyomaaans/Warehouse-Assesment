package products

import (
	"warehouse/internal/products/domain"
	"warehouse/pkg"
)

func ToProductResponse(e domain.ProductEntity) ProductResponse {
	return ProductResponse{ID: pkg.EncodeID(e.ID), SKU: e.SKU, Name: e.Name, CreatedAt: e.CreatedAt}
}
