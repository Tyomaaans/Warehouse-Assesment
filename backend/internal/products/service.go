package products

import (
	"context"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"warehouse/internal/products/domain"
	"warehouse/pkg"
)

// Interface
type ProductService interface {
	CreateProduct(context.Context, CreateProductRequest) (*ProductResponse, error)
	GetProducts(context.Context, string, int, int) (*PaginatedProductResponse, error)
}

// Implementation
type productService struct {
	productRepo domain.ProductRepository
	validate    *validator.Validate
}

func NewProductService(repo domain.ProductRepository, validate *validator.Validate) ProductService {
	return &productService{productRepo: repo, validate: validate}
}

func (s *productService) CreateProduct(ctx context.Context, req CreateProductRequest) (*ProductResponse, error) {
	if _, err := pkg.RequireActor(ctx, pkg.Supervisor); err != nil {
		return nil, err
	}
	if err := pkg.ValidateStruct(s.validate, req); err != nil {
		return nil, err
	}
	e := domain.ProductEntity{ID: uuid.New(), SKU: req.SKU, Name: req.Name, CreatedAt: time.Now().UTC()}
	if err := s.productRepo.CreateProduct(ctx, &e); err != nil {
		return nil, err
	}
	response := ToProductResponse(e)
	return &response, nil
}

func (s *productService) GetProducts(ctx context.Context, search string, page, limit int) (*PaginatedProductResponse, error) {
	if _, err := pkg.RequireActor(ctx, ""); err != nil {
		return nil, err
	}
	if err := pkg.ValidatePagination(page, limit); err != nil {
		return nil, err
	}
	if err := pkg.ValidateSearch(search); err != nil {
		return nil, err
	}
	list, total, err := s.productRepo.GetProducts(ctx, search, page, limit)
	if err != nil {
		return nil, err
	}
	result := &PaginatedProductResponse{Products: make([]ProductResponse, len(list)), Meta: pkg.NewMeta(page, limit, total)}
	for i, e := range list {
		result.Products[i] = ToProductResponse(e)
	}
	return result, nil
}
