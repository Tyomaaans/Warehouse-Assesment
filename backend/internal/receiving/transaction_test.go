package receiving

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"warehouse/internal/receiving/domain"
	"warehouse/pkg"
)

type receivingTxStub struct {
	document    *domain.DocumentEntity
	transaction *domain.TransactionEntity
	active      bool
	saved       bool
	posted      bool
}

func (r *receivingTxStub) Document() *domain.DocumentEntity    { return r.document }
func (r *receivingTxStub) HasActiveTransaction() (bool, error) { return r.active, nil }
func (r *receivingTxStub) GetTransaction(uuid.UUID) (*domain.TransactionEntity, error) {
	return r.transaction, nil
}
func (r *receivingTxStub) SaveTransaction(e *domain.TransactionEntity, _, _ bool) error {
	r.saved, r.transaction = true, e
	return nil
}
func (r *receivingTxStub) PostInventory(e *domain.TransactionEntity) error {
	if e.Status != domain.Approved || e.ApprovedBy == nil || e.ApprovedAt == nil {
		return pkg.ErrInternal
	}
	r.posted = true
	return nil
}
func (r *receivingTxStub) CompleteDocument(now time.Time) error {
	r.document.Status, r.document.CompletedAt = domain.Completed, &now
	return nil
}
func (*receivingTxStub) AppendAudit(domain.AuditEntity) error              { return nil }
func (*receivingTxStub) AddOutbox(string, *domain.TransactionEntity) error { return nil }

func TestTransactionServiceRoleBoundaries(t *testing.T) {
	service := NewReceivingService(nil, pkg.NewValidator())
	op := pkg.WithActor(context.Background(), pkg.Actor{ID: uuid.New(), Role: pkg.Operator})
	spv := pkg.WithActor(context.Background(), pkg.Actor{ID: uuid.New(), Role: pkg.Supervisor})
	id := pkg.EncodeID(uuid.New())
	if _, err := service.CreateTransaction(spv, id, CreateTransactionRequest{}); !errors.Is(err, pkg.ErrForbidden) {
		t.Fatalf("Supervisor submit error = %v", err)
	}
	if _, err := service.ApproveTransaction(op, id); !errors.Is(err, pkg.ErrForbidden) {
		t.Fatalf("Operator approve error = %v", err)
	}
	if _, err := service.CorrectTransaction(op, id, CorrectTransactionRequest{}); !errors.Is(err, pkg.ErrForbidden) {
		t.Fatalf("Operator correct error = %v", err)
	}
	if _, err := service.ReviewTransaction(op, id, ReviewTransactionRequest{}); !errors.Is(err, pkg.ErrForbidden) {
		t.Fatalf("Operator review error = %v", err)
	}
	if _, err := service.GetAuditLogs(op, "", 1, 10); !errors.Is(err, pkg.ErrForbidden) {
		t.Fatalf("Operator audit error = %v", err)
	}
	if _, err := service.GetTransaction(context.Background(), id); !errors.Is(err, pkg.ErrUnauthorized) {
		t.Fatalf("anonymous detail error = %v", err)
	}
	if _, err := service.GetTransactions(context.Background(), id, 1, 10); !errors.Is(err, pkg.ErrUnauthorized) {
		t.Fatalf("anonymous history error = %v", err)
	}
}

func TestApprovalRevalidatesCurrentRemaining(t *testing.T) {
	docID, lineID := uuid.New(), uuid.New()
	e := &domain.TransactionEntity{ID: uuid.New(), ReceivingDocumentID: docID, Status: domain.Pending, Items: []domain.TransactionItemEntity{
		{ID: uuid.New(), ReceivingDocumentItemID: lineID, ActualQuantity: 3},
	}}
	tx := &receivingTxStub{transaction: e, document: &domain.DocumentEntity{ID: docID, Status: domain.Incomplete, Items: []domain.DocumentItemEntity{
		{ID: lineID, ExpectedQuantity: 10, ApprovedQuantity: 8},
	}}}
	service := NewReceivingService(&receivingRepositoryStub{transaction: e, tx: tx}, pkg.NewValidator())
	ctx := pkg.WithActor(context.Background(), pkg.Actor{ID: uuid.New(), Role: pkg.Supervisor})
	if _, err := service.ApproveTransaction(ctx, pkg.EncodeID(e.ID)); !errors.Is(err, pkg.ErrConflict) || tx.saved || tx.posted {
		t.Fatalf("over-cap approval error = %v, saved=%v posted=%v", err, tx.saved, tx.posted)
	}
	e.Items[0].ActualQuantity = 2
	if _, err := service.ApproveTransaction(ctx, pkg.EncodeID(e.ID)); err != nil || !tx.saved || !tx.posted || tx.document.Status != domain.Completed {
		t.Fatalf("valid approval error = %v", err)
	}
}

func TestCorrectionPolicyAndRetainedItemIdentity(t *testing.T) {
	docID, lineID, itemID := uuid.New(), uuid.New(), uuid.New()
	state := domain.Unresolved
	e := &domain.TransactionEntity{ID: uuid.New(), ReceivingDocumentID: docID, Status: domain.Rejected, ReviewStatus: &state, Items: []domain.TransactionItemEntity{
		{ID: itemID, ReceivingDocumentItemID: lineID, ActualQuantity: 60},
	}}
	tx := &receivingTxStub{transaction: e, document: &domain.DocumentEntity{ID: docID, Status: domain.Incomplete, Items: []domain.DocumentItemEntity{{ID: lineID, ExpectedQuantity: 10}}}}
	service := NewReceivingService(&receivingRepositoryStub{transaction: e, tx: tx}, pkg.NewValidator())
	ctx := pkg.WithActor(context.Background(), pkg.Actor{ID: uuid.New(), Role: pkg.Supervisor})
	req := CorrectTransactionRequest{Reason: "correct input", Items: []ActualItemRequest{{ReceivingDocumentItemID: pkg.EncodeID(lineID), ActualQuantity: 10}}}
	result, err := service.CorrectTransaction(ctx, pkg.EncodeID(e.ID), req)
	if err != nil || result.ID != pkg.EncodeID(e.ID) || result.Items[0].ID != pkg.EncodeID(itemID) || e.Status != domain.Pending || e.ReviewStatus != nil || tx.posted {
		t.Fatalf("correction error = %v", err)
	}
	if _, err := service.CorrectTransaction(ctx, pkg.EncodeID(e.ID), req); !errors.Is(err, pkg.ErrConflict) {
		t.Fatalf("pending correction error = %v, want conflict", err)
	}
}
