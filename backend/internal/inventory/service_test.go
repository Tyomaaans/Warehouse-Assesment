package inventory

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"warehouse/internal/inventory/domain"
	"warehouse/pkg"
)

type inventoryRepositoryStub struct{}

func (*inventoryRepositoryStub) GetBalances(context.Context, string, int, int) ([]domain.BalanceEntity, int64, error) {
	return nil, 0, nil
}
func (*inventoryRepositoryStub) GetMovements(context.Context, *uuid.UUID, int, int) ([]domain.MovementEntity, int64, error) {
	return nil, 0, nil
}

func TestInventoryServiceAuthorizationAndValidation(t *testing.T) {
	service := NewInventoryService(&inventoryRepositoryStub{})
	if _, err := service.GetBalances(context.Background(), "", 1, 10); !errors.Is(err, pkg.ErrUnauthorized) {
		t.Fatalf("anonymous balances error = %v", err)
	}
	if _, err := service.GetMovements(context.Background(), "", 1, 10); !errors.Is(err, pkg.ErrUnauthorized) {
		t.Fatalf("anonymous movements error = %v", err)
	}
	for _, role := range []string{pkg.Operator, pkg.Supervisor} {
		ctx := pkg.WithActor(context.Background(), pkg.Actor{ID: uuid.New(), Role: role})
		balances, err := service.GetBalances(ctx, "", 1, 10)
		if err != nil || balances.Balances == nil || balances.Meta.TotalPages != 0 {
			t.Fatalf("balances error = %v", err)
		}
		movements, err := service.GetMovements(ctx, "", 1, 10)
		if err != nil || movements.Movements == nil {
			t.Fatalf("movements error = %v", err)
		}
		if _, err := service.GetMovements(ctx, uuid.NewString(), 1, 10); !errors.Is(err, pkg.ErrInvalidInput) {
			t.Fatalf("text UUID error = %v", err)
		}
		if _, err := service.GetBalances(ctx, "", 1, 0); !errors.Is(err, pkg.ErrInvalidInput) {
			t.Fatalf("zero limit error = %v", err)
		}
	}
}
