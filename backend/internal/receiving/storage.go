package receiving

import (
	"time"

	"github.com/google/uuid"
)

type DocumentStorage struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrderReference string
	Status         string
	CreatedBy      uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

func (DocumentStorage) TableName() string { return "receiving_documents" }

type DocumentItemStorage struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey"`
	ReceivingDocumentID uuid.UUID
	ProductID           uuid.UUID
	ExpectedQuantity    int64
}

func (DocumentItemStorage) TableName() string { return "receiving_document_items" }

type TransactionStorage struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey"`
	ReceivingDocumentID uuid.UUID
	Status              string
	ReviewStatus        *string
	RejectionReason     *string
	ReviewNote          *string
	CreatedBy           uuid.UUID
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ApprovedBy          *uuid.UUID
	ApprovedAt          *time.Time
	CorrectedBy         *uuid.UUID
	CorrectedAt         *time.Time
	ReviewedBy          *uuid.UUID
	ReviewedAt          *time.Time
}

func (TransactionStorage) TableName() string { return "receiving_transactions" }

type TransactionItemStorage struct {
	ID                      uuid.UUID `gorm:"type:uuid;primaryKey"`
	ReceivingTransactionID  uuid.UUID
	ReceivingDocumentItemID uuid.UUID
	ActualQuantity          int64
}

func (TransactionItemStorage) TableName() string { return "receiving_transaction_items" }
