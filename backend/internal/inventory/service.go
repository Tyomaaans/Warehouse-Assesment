package inventory

import (
	"context"

	"github.com/google/uuid"

	"warehouse/internal/inventory/domain"
	"warehouse/pkg"
)

// Interface
type InventoryService interface {
	GetBalances(context.Context, string, int, int) (*PaginatedBalanceResponse, error)
	GetMovements(context.Context, string, int, int) (*PaginatedMovementResponse, error)
}

// Implementation
type inventoryService struct {
	inventoryRepo domain.InventoryRepository
}

func NewInventoryService(inventoryRepo domain.InventoryRepository) InventoryService {
	return &inventoryService{inventoryRepo: inventoryRepo}
}

func (s *inventoryService) GetBalances(ctx context.Context, search string, page, limit int) (*PaginatedBalanceResponse, error) {
	if _, err := pkg.RequireActor(ctx, ""); err != nil {
		return nil, err
	}
	if err := pkg.ValidatePagination(page, limit); err != nil {
		return nil, err
	}
	if err := pkg.ValidateSearch(search); err != nil {
		return nil, err
	}
	rows, total, err := s.inventoryRepo.GetBalances(ctx, search, page, limit)
	if err != nil {
		return nil, err
	}
	result := &PaginatedBalanceResponse{Balances: make([]BalanceResponse, len(rows)), Meta: pkg.NewMeta(page, limit, total)}
	for i, row := range rows {
		result.Balances[i] = ToBalanceResponse(row)
	}
	return result, nil
}

func (s *inventoryService) GetMovements(ctx context.Context, rawID string, page, limit int) (*PaginatedMovementResponse, error) {
	if _, err := pkg.RequireActor(ctx, ""); err != nil {
		return nil, err
	}
	if err := pkg.ValidatePagination(page, limit); err != nil {
		return nil, err
	}
	var id *uuid.UUID
	if rawID != "" {
		parsed, err := pkg.ParseID(rawID)
		if err != nil {
			return nil, err
		}
		id = &parsed
	}
	rows, total, err := s.inventoryRepo.GetMovements(ctx, id, page, limit)
	if err != nil {
		return nil, err
	}
	result := &PaginatedMovementResponse{Movements: make([]MovementResponse, len(rows)), Meta: pkg.NewMeta(page, limit, total)}
	for i, row := range rows {
		result.Movements[i] = ToMovementResponse(row)
	}
	return result, nil
}
