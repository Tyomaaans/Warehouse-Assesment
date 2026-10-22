package receiving

import (
	"time"

	"warehouse/pkg"
)

type ExpectedItemRequest struct {
	ProductID        string `json:"product_id" validate:"required,base64uuid4"`
	ExpectedQuantity int64  `json:"expected_quantity" validate:"gt=0"`
}

type CreateReceivingRequest struct {
	OrderReference string                `json:"order_reference" validate:"required,notblank"`
	Items          []ExpectedItemRequest `json:"items" validate:"required,min=1,dive"`
}

type ActualItemRequest struct {
	ReceivingDocumentItemID string `json:"receiving_document_item_id" validate:"required,base64uuid4"`
	ActualQuantity          int64  `json:"actual_quantity" validate:"gt=0"`
}

type CreateTransactionRequest struct {
	Items []ActualItemRequest `json:"items" validate:"required,min=1,dive"`
}

type CorrectTransactionRequest struct {
	Reason string              `json:"reason" validate:"required,notblank"`
	Items  []ActualItemRequest `json:"items" validate:"required,min=1,dive"`
}

type ReviewTransactionRequest struct {
	Note string `json:"note" validate:"required,notblank"`
}

type ReceivingSummary struct {
	ID             string     `json:"id"`
	OrderReference string     `json:"order_reference"`
	Status         string     `json:"status"`
	CreatedBy      string     `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

type ReceivingItemResponse struct {
	ID                string `json:"id"`
	ProductID         string `json:"product_id"`
	SKU               string `json:"sku"`
	Name              string `json:"name"`
	ExpectedQuantity  int64  `json:"expected_quantity"`
	ApprovedQuantity  int64  `json:"approved_quantity"`
	RemainingQuantity int64  `json:"remaining_quantity"`
}

type ReceivingResponse struct {
	ReceivingSummary
	Items []ReceivingItemResponse `json:"items"`
}

type TransactionItemResponse struct {
	ID                      string `json:"id"`
	ReceivingDocumentItemID string `json:"receiving_document_item_id"`
	ActualQuantity          int64  `json:"actual_quantity"`
}

type TransactionResponse struct {
	ID                  string                    `json:"id"`
	ReceivingDocumentID string                    `json:"receiving_document_id"`
	Status              string                    `json:"status"`
	ReviewStatus        *string                   `json:"review_status"`
	RejectionReason     *string                   `json:"rejection_reason"`
	ReviewNote          *string                   `json:"review_note"`
	CreatedBy           string                    `json:"created_by"`
	CreatedAt           time.Time                 `json:"created_at"`
	UpdatedAt           time.Time                 `json:"updated_at"`
	ApprovedBy          *string                   `json:"approved_by"`
	ApprovedAt          *time.Time                `json:"approved_at"`
	CorrectedBy         *string                   `json:"corrected_by"`
	CorrectedAt         *time.Time                `json:"corrected_at"`
	ReviewedBy          *string                   `json:"reviewed_by"`
	ReviewedAt          *time.Time                `json:"reviewed_at"`
	Items               []TransactionItemResponse `json:"items"`
}

type PaginatedReceivingResponse struct {
	Receivings []ReceivingSummary `json:"receivings"`
	Meta       pkg.PaginationMeta `json:"meta"`
}

type PaginatedTransactionResponse struct {
	Transactions []TransactionResponse `json:"transactions"`
	Meta         pkg.PaginationMeta    `json:"meta"`
}
