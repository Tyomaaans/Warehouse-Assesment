package domain

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	Incomplete = "INCOMPLETE"
	Completed  = "COMPLETED"
	Pending    = "PENDING_APPROVAL"
	Approved   = "APPROVED"
	Rejected   = "REJECTED_BY_SYSTEM"
	Unresolved = "UNRESOLVED"
	Resolved   = "RESOLVED"
)

type DocumentItemEntity struct {
	ID                  uuid.UUID
	ReceivingDocumentID uuid.UUID
	ProductID           uuid.UUID
	SKU                 string
	Name                string
	ExpectedQuantity    int64
	ApprovedQuantity    int64
}

type DocumentEntity struct {
	ID             uuid.UUID
	OrderReference string
	Status         string
	CreatedBy      uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
	Items          []DocumentItemEntity
}

type TransactionItemEntity struct {
	ID                      uuid.UUID
	ReceivingTransactionID  uuid.UUID
	ReceivingDocumentItemID uuid.UUID
	ActualQuantity          int64
}

type TransactionEntity struct {
	ID                  uuid.UUID
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
	Items               []TransactionItemEntity
}

type AuditEntity struct {
	DocumentID    uuid.UUID
	TransactionID *uuid.UUID
	ActorID       *uuid.UUID
	EventType     string
	Before        any
	After         any
	Reason        *string
	CreatedAt     time.Time
}

type AuditRecordEntity struct {
	ID                     uuid.UUID
	ReceivingDocumentID    uuid.UUID
	ReceivingTransactionID *uuid.UUID
	ActorID                *uuid.UUID
	EventType              string
	BeforeData             json.RawMessage
	AfterData              json.RawMessage
	Reason                 *string
	CreatedAt              time.Time
}

type ReceivingTx interface {
	Document() *DocumentEntity
	HasActiveTransaction() (bool, error)
	GetTransaction(uuid.UUID) (*TransactionEntity, error)
	SaveTransaction(*TransactionEntity, bool, bool) error
	PostInventory(*TransactionEntity) error
	CompleteDocument(time.Time) error
	AppendAudit(AuditEntity) error
	AddOutbox(string, *TransactionEntity) error
}

type ReceivingRepository interface {
	CreateDocument(context.Context, *DocumentEntity, AuditEntity) error
	GetDocuments(context.Context, string, int, int) ([]DocumentEntity, int64, error)
	GetDocument(context.Context, uuid.UUID) (*DocumentEntity, error)
	GetTransactions(context.Context, uuid.UUID, int, int) ([]TransactionEntity, int64, error)
	GetTransaction(context.Context, uuid.UUID) (*TransactionEntity, error)
	WithDocument(context.Context, uuid.UUID, func(ReceivingTx) error) error
	GetAuditLogs(context.Context, *uuid.UUID, int, int) ([]AuditRecordEntity, int64, error)
}
