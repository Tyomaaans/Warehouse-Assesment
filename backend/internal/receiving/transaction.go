package receiving

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"warehouse/internal/receiving/domain"
	"warehouse/pkg"
)

func (s *receivingService) CreateTransaction(ctx context.Context, rawID string, req CreateTransactionRequest) (*TransactionResponse, error) {
	actor, err := pkg.RequireActor(ctx, pkg.Operator)
	if err != nil {
		return nil, err
	}
	id, err := pkg.ParseID(rawID)
	if err != nil {
		return nil, err
	}
	if err := pkg.ValidateStruct(s.validate, req); err != nil {
		return nil, err
	}
	var response *TransactionResponse
	err = s.receivingRepo.WithDocument(ctx, id, func(tx domain.ReceivingTx) error {
		if tx.Document().Status != domain.Incomplete {
			return fmt.Errorf("%w: completed receiving documents cannot accept transactions", pkg.ErrConflict)
		}
		active, err := tx.HasActiveTransaction()
		if err != nil {
			return err
		}
		if active {
			return fmt.Errorf("%w: receiving document already has an active transaction", pkg.ErrConflict)
		}
		now := time.Now().UTC()
		e := &domain.TransactionEntity{
			ID: uuid.New(), ReceivingDocumentID: id, CreatedBy: actor.ID,
			CreatedAt: now, UpdatedAt: now,
		}
		items, reason, err := transactionItems(tx.Document(), e, req.Items)
		if err != nil {
			return err
		}
		e.Items = items
		setTransactionState(e, reason)
		if err := tx.SaveTransaction(e, true, true); err != nil {
			return err
		}
		if err := transactionAudit(tx, e, actor, "TRANSACTION_SUBMITTED", nil, nil); err != nil {
			return err
		}
		event := "TRANSACTION_SUBMITTED"
		if reason != nil {
			event = "TRANSACTION_REJECTED"
			if err := transactionAudit(tx, e, actor, event, nil, reason); err != nil {
				return err
			}
		}
		if err := tx.AddOutbox(event, e); err != nil {
			return err
		}
		response = ToTransactionResponse(e)
		return nil
	})
	return response, err
}

func (s *receivingService) GetTransactions(ctx context.Context, rawID string, page, limit int) (*PaginatedTransactionResponse, error) {
	if _, err := pkg.RequireActor(ctx, ""); err != nil {
		return nil, err
	}
	id, err := pkg.ParseID(rawID)
	if err != nil {
		return nil, err
	}
	if err := pkg.ValidatePagination(page, limit); err != nil {
		return nil, err
	}
	list, total, err := s.receivingRepo.GetTransactions(ctx, id, page, limit)
	if err != nil {
		return nil, err
	}
	result := &PaginatedTransactionResponse{Transactions: make([]TransactionResponse, len(list)), Meta: pkg.NewMeta(page, limit, total)}
	for i := range list {
		result.Transactions[i] = *ToTransactionResponse(&list[i])
	}
	return result, nil
}

func (s *receivingService) GetTransaction(ctx context.Context, rawID string) (*TransactionResponse, error) {
	if _, err := pkg.RequireActor(ctx, ""); err != nil {
		return nil, err
	}
	id, err := pkg.ParseID(rawID)
	if err != nil {
		return nil, err
	}
	e, err := s.receivingRepo.GetTransaction(ctx, id)
	if err != nil {
		return nil, err
	}
	return ToTransactionResponse(e), nil
}

func (s *receivingService) CorrectTransaction(ctx context.Context, rawID string, req CorrectTransactionRequest) (*TransactionResponse, error) {
	actor, err := pkg.RequireActor(ctx, pkg.Supervisor)
	if err != nil {
		return nil, err
	}
	if err := pkg.ValidateStruct(s.validate, req); err != nil {
		return nil, err
	}
	return s.changeTransaction(ctx, rawID, func(tx domain.ReceivingTx, e *domain.TransactionEntity) error {
		if e.Status != domain.Rejected || e.ReviewStatus == nil || *e.ReviewStatus != domain.Unresolved {
			return fmt.Errorf("%w: only unresolved rejected transactions can be corrected", pkg.ErrConflict)
		}
		before := ToTransactionResponse(e)
		items, reason, err := transactionItems(tx.Document(), e, req.Items)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		e.Items, e.UpdatedAt, e.CorrectedBy, e.CorrectedAt = items, now, &actor.ID, &now
		setTransactionState(e, reason)
		if err := tx.SaveTransaction(e, false, true); err != nil {
			return err
		}
		if err := transactionAudit(tx, e, actor, "TRANSACTION_CORRECTED", before, &req.Reason); err != nil {
			return err
		}
		if reason != nil {
			if err := transactionAudit(tx, e, actor, "TRANSACTION_REJECTED", before, reason); err != nil {
				return err
			}
		}
		return tx.AddOutbox("TRANSACTION_CORRECTED", e)
	})
}

func (s *receivingService) ReviewTransaction(ctx context.Context, rawID string, req ReviewTransactionRequest) (*TransactionResponse, error) {
	actor, err := pkg.RequireActor(ctx, pkg.Supervisor)
	if err != nil {
		return nil, err
	}
	if err := pkg.ValidateStruct(s.validate, req); err != nil {
		return nil, err
	}
	return s.changeTransaction(ctx, rawID, func(tx domain.ReceivingTx, e *domain.TransactionEntity) error {
		if e.Status != domain.Rejected || e.ReviewStatus == nil || *e.ReviewStatus != domain.Unresolved {
			return fmt.Errorf("%w: only unresolved rejected transactions can be reviewed", pkg.ErrConflict)
		}
		before := ToTransactionResponse(e)
		now, resolved := time.Now().UTC(), domain.Resolved
		e.ReviewStatus, e.ReviewNote, e.ReviewedBy, e.ReviewedAt, e.UpdatedAt = &resolved, &req.Note, &actor.ID, &now, now
		if err := tx.SaveTransaction(e, false, false); err != nil {
			return err
		}
		if err := transactionAudit(tx, e, actor, "REVIEW_RESOLVED", before, &req.Note); err != nil {
			return err
		}
		return tx.AddOutbox("REVIEW_RESOLVED", e)
	})
}

func (s *receivingService) ApproveTransaction(ctx context.Context, rawID string) (*TransactionResponse, error) {
	actor, err := pkg.RequireActor(ctx, pkg.Supervisor)
	if err != nil {
		return nil, err
	}
	return s.changeTransaction(ctx, rawID, func(tx domain.ReceivingTx, e *domain.TransactionEntity) error {
		if e.Status != domain.Pending {
			return fmt.Errorf("%w: only pending transactions can be approved", pkg.ErrConflict)
		}
		actuals := make([]ActualItemRequest, len(e.Items))
		for i, item := range e.Items {
			actuals[i] = ActualItemRequest{ReceivingDocumentItemID: pkg.EncodeID(item.ReceivingDocumentItemID), ActualQuantity: item.ActualQuantity}
		}
		_, reason, err := transactionItems(tx.Document(), e, actuals)
		if err != nil {
			return err
		}
		if reason != nil {
			return fmt.Errorf("%w: transaction exceeds remaining expected quantity", pkg.ErrConflict)
		}
		before := ToTransactionResponse(e)
		documentBefore := ToReceivingResponse(tx.Document())
		now := time.Now().UTC()
		e.Status, e.ApprovedBy, e.ApprovedAt, e.UpdatedAt = domain.Approved, &actor.ID, &now, now
		if err := tx.SaveTransaction(e, false, false); err != nil {
			return err
		}
		if err := tx.PostInventory(e); err != nil {
			return err
		}
		if err := transactionAudit(tx, e, actor, "TRANSACTION_APPROVED", before, nil); err != nil {
			return err
		}
		quantities := make(map[uuid.UUID]int64, len(e.Items))
		for _, item := range e.Items {
			quantities[item.ReceivingDocumentItemID] = item.ActualQuantity
		}
		complete := true
		for i := range tx.Document().Items {
			item := &tx.Document().Items[i]
			item.ApprovedQuantity += quantities[item.ID]
			if item.ApprovedQuantity != item.ExpectedQuantity {
				complete = false
			}
		}
		if complete {
			if err := tx.CompleteDocument(now); err != nil {
				return err
			}
			if err := tx.AppendAudit(domain.AuditEntity{
				DocumentID: e.ReceivingDocumentID, TransactionID: &e.ID, ActorID: &actor.ID,
				EventType: "RECEIVING_COMPLETED", Before: documentBefore, After: ToReceivingResponse(tx.Document()), CreatedAt: now,
			}); err != nil {
				return err
			}
		}
		return tx.AddOutbox("TRANSACTION_APPROVED", e)
	})
}

func (s *receivingService) changeTransaction(ctx context.Context, rawID string, change func(domain.ReceivingTx, *domain.TransactionEntity) error) (*TransactionResponse, error) {
	id, err := pkg.ParseID(rawID)
	if err != nil {
		return nil, err
	}
	current, err := s.receivingRepo.GetTransaction(ctx, id)
	if err != nil {
		return nil, err
	}
	var response *TransactionResponse
	err = s.receivingRepo.WithDocument(ctx, current.ReceivingDocumentID, func(tx domain.ReceivingTx) error {
		if tx.Document().Status != domain.Incomplete {
			return fmt.Errorf("%w: receiving document is completed", pkg.ErrConflict)
		}
		e, err := tx.GetTransaction(id)
		if err != nil {
			return err
		}
		if err := change(tx, e); err != nil {
			return err
		}
		response = ToTransactionResponse(e)
		return nil
	})
	return response, err
}

func transactionItems(document *domain.DocumentEntity, transaction *domain.TransactionEntity, requested []ActualItemRequest) ([]domain.TransactionItemEntity, *string, error) {
	lines := make(map[uuid.UUID]domain.DocumentItemEntity, len(document.Items))
	for _, item := range document.Items {
		lines[item.ID] = item
	}
	retained := make(map[uuid.UUID]uuid.UUID, len(transaction.Items))
	for _, item := range transaction.Items {
		retained[item.ReceivingDocumentItemID] = item.ID
	}
	seen := make(map[uuid.UUID]bool, len(requested))
	items := make([]domain.TransactionItemEntity, len(requested))
	var rejection *string
	for i, item := range requested {
		id, err := pkg.ParseID(item.ReceivingDocumentItemID)
		if err != nil {
			return nil, nil, err
		}
		line, exists := lines[id]
		if !exists || seen[id] || item.ActualQuantity <= 0 {
			return nil, nil, fmt.Errorf("%w: items[%d] must have a unique line belonging to this receiving document and a positive actual_quantity", pkg.ErrInvalidInput, i)
		}
		seen[id] = true
		itemID := retained[id]
		if itemID == uuid.Nil {
			itemID = uuid.New()
		}
		items[i] = domain.TransactionItemEntity{ID: itemID, ReceivingTransactionID: transaction.ID, ReceivingDocumentItemID: id, ActualQuantity: item.ActualQuantity}
		if item.ActualQuantity > line.ExpectedQuantity-line.ApprovedQuantity {
			reason := fmt.Sprintf("items[%d].actual_quantity exceeds remaining expected quantity", i)
			rejection = &reason
		}
	}
	if len(items) == 0 {
		return nil, nil, fmt.Errorf("%w: items must contain at least one SKU line", pkg.ErrInvalidInput)
	}
	return items, rejection, nil
}

func setTransactionState(e *domain.TransactionEntity, reason *string) {
	e.Status, e.ReviewStatus, e.RejectionReason = domain.Pending, nil, nil
	if reason != nil {
		unresolved := domain.Unresolved
		e.Status, e.ReviewStatus, e.RejectionReason = domain.Rejected, &unresolved, reason
	}
}

func transactionAudit(tx domain.ReceivingTx, e *domain.TransactionEntity, actor pkg.Actor, event string, before any, reason *string) error {
	return tx.AppendAudit(domain.AuditEntity{
		DocumentID: e.ReceivingDocumentID, TransactionID: &e.ID, ActorID: &actor.ID,
		EventType: event, Before: before, After: ToTransactionResponse(e), Reason: reason, CreatedAt: e.UpdatedAt,
	})
}
