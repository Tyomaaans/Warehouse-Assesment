package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type BalanceEntity struct {
	ID        uuid.UUID
	ProductID uuid.UUID
	SKU       string
	Name      string
	Quantity  int64
	UpdatedAt time.Time
}

type MovementEntity struct {
	ID                         uuid.UUID
	ProductID                  uuid.UUID
	ReceivingTransactionItemID uuid.UUID
	ReceivingTransactionID     uuid.UUID
	ReceivingDocumentID        uuid.UUID
	QuantityDelta              int64
	CreatedAt                  time.Time
}

type InventoryRepository interface {
	GetBalances(context.Context, string, int, int) ([]BalanceEntity, int64, error)
	GetMovements(context.Context, *uuid.UUID, int, int) ([]MovementEntity, int64, error)
}
