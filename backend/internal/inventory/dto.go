package inventory

import (
	"time"

	"warehouse/pkg"
)

type BalanceResponse struct {
	ID        string    `json:"id"`
	ProductID string    `json:"product_id"`
	SKU       string    `json:"sku"`
	Name      string    `json:"name"`
	Quantity  int64     `json:"quantity"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MovementResponse struct {
	ID                         string    `json:"id"`
	ProductID                  string    `json:"product_id"`
	ReceivingTransactionItemID string    `json:"receiving_transaction_item_id"`
	ReceivingTransactionID     string    `json:"receiving_transaction_id"`
	ReceivingDocumentID        string    `json:"receiving_document_id"`
	QuantityDelta              int64     `json:"quantity_delta"`
	CreatedAt                  time.Time `json:"created_at"`
}

type PaginatedBalanceResponse struct {
	Balances []BalanceResponse  `json:"balances"`
	Meta     pkg.PaginationMeta `json:"meta"`
}

type PaginatedMovementResponse struct {
	Movements []MovementResponse `json:"movements"`
	Meta      pkg.PaginationMeta `json:"meta"`
}
