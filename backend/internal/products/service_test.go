package products

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"warehouse/internal/products/domain"
	"warehouse/pkg"
)

type productRepositoryStub struct {
	created *domain.ProductEntity
	err     error
}

func (r *productRepositoryStub) CreateProduct(_ context.Context, product *domain.ProductEntity) error {
	r.created = product
	return r.err
}

func (r *productRepositoryStub) GetProducts(context.Context, string, int, int) ([]domain.ProductEntity, int64, error) {
	return nil, 0, r.err
}

func TestProductServiceAuthorization(t *testing.T) {
	repo := &productRepositoryStub{}
	service := NewProductService(repo, pkg.NewValidator())
	req := CreateProductRequest{SKU: "SKU-A", Name: "Product A"}
	if _, err := service.CreateProduct(context.Background(), req); !errors.Is(err, pkg.ErrUnauthorized) {
		t.Fatalf("anonymous create error = %v, want unauthorized", err)
	}
	operator := pkg.WithActor(context.Background(), pkg.Actor{ID: uuid.New(), SessionID: uuid.New(), Role: pkg.Operator})
	if _, err := service.CreateProduct(operator, req); !errors.Is(err, pkg.ErrForbidden) || repo.created != nil {
		t.Fatalf("Operator create error = %v, want forbidden before persistence", err)
	}
	supervisor := pkg.WithActor(context.Background(), pkg.Actor{ID: uuid.New(), SessionID: uuid.New(), Role: pkg.Supervisor})
	response, err := service.CreateProduct(supervisor, req)
	if err != nil || repo.created == nil {
		t.Fatalf("Supervisor create error = %v", err)
	}
	if _, err := pkg.ParseID(response.ID); err != nil || response.SKU != req.SKU || response.CreatedAt.IsZero() {
		t.Fatal("invalid product identity or fields")
	}
	if _, err := service.GetProducts(context.Background(), "", 1, 10); !errors.Is(err, pkg.ErrUnauthorized) {
		t.Fatalf("anonymous read error = %v, want unauthorized", err)
	}
	for _, ctx := range []context.Context{operator, supervisor} {
		page, err := service.GetProducts(ctx, "", 1, 10)
		if err != nil || page.Products == nil || page.Meta.TotalPages != 0 {
			t.Fatalf("authenticated empty read error = %v", err)
		}
	}
}

func TestProductServiceValidationAndConflict(t *testing.T) {
	repo := &productRepositoryStub{}
	service := NewProductService(repo, pkg.NewValidator())
	ctx := pkg.WithActor(context.Background(), pkg.Actor{ID: uuid.New(), SessionID: uuid.New(), Role: pkg.Supervisor})
	for _, req := range []CreateProductRequest{{Name: "A"}, {SKU: " \t", Name: "A"}, {SKU: "A"}, {SKU: "A", Name: "\u0000"}} {
		if _, err := service.CreateProduct(ctx, req); !errors.Is(err, pkg.ErrInvalidInput) || repo.created != nil {
			t.Fatalf("invalid product error = %v, want invalid input before persistence", err)
		}
	}
	if _, err := service.GetProducts(ctx, "", 1, 0); !errors.Is(err, pkg.ErrInvalidInput) {
		t.Fatalf("zero limit error = %v, want invalid input", err)
	}
	repo.err = pkg.ErrAlreadyExists
	if _, err := service.CreateProduct(ctx, CreateProductRequest{SKU: "A", Name: "A"}); !errors.Is(err, pkg.ErrAlreadyExists) {
		t.Fatalf("duplicate SKU error = %v, want already exists", err)
	}
}
