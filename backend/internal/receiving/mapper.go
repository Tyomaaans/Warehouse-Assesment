package receiving

import (
	"warehouse/internal/receiving/domain"
	"warehouse/pkg"
)

func ToReceivingSummary(e *domain.DocumentEntity) ReceivingSummary {
	return ReceivingSummary{ID: pkg.EncodeID(e.ID), OrderReference: e.OrderReference, Status: e.Status, CreatedBy: pkg.EncodeID(e.CreatedBy), CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, CompletedAt: e.CompletedAt}
}

func ToReceivingResponse(e *domain.DocumentEntity) *ReceivingResponse {
	result := &ReceivingResponse{ReceivingSummary: ToReceivingSummary(e), Items: make([]ReceivingItemResponse, len(e.Items))}
	for i, item := range e.Items {
		result.Items[i] = ReceivingItemResponse{ID: pkg.EncodeID(item.ID), ProductID: pkg.EncodeID(item.ProductID), SKU: item.SKU, Name: item.Name, ExpectedQuantity: item.ExpectedQuantity, ApprovedQuantity: item.ApprovedQuantity, RemainingQuantity: item.ExpectedQuantity - item.ApprovedQuantity}
	}
	return result
}

func ToTransactionResponse(e *domain.TransactionEntity) *TransactionResponse {
	result := &TransactionResponse{ID: pkg.EncodeID(e.ID), ReceivingDocumentID: pkg.EncodeID(e.ReceivingDocumentID), Status: e.Status, ReviewStatus: e.ReviewStatus, RejectionReason: e.RejectionReason, ReviewNote: e.ReviewNote, CreatedBy: pkg.EncodeID(e.CreatedBy), CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, ApprovedBy: pkg.EncodeNullableID(e.ApprovedBy), ApprovedAt: e.ApprovedAt, CorrectedBy: pkg.EncodeNullableID(e.CorrectedBy), CorrectedAt: e.CorrectedAt, ReviewedBy: pkg.EncodeNullableID(e.ReviewedBy), ReviewedAt: e.ReviewedAt, Items: make([]TransactionItemResponse, len(e.Items))}
	for i, item := range e.Items {
		result.Items[i] = TransactionItemResponse{ID: pkg.EncodeID(item.ID), ReceivingDocumentItemID: pkg.EncodeID(item.ReceivingDocumentItemID), ActualQuantity: item.ActualQuantity}
	}
	return result
}

func ToDocumentEntity(s DocumentStorage) domain.DocumentEntity {
	return domain.DocumentEntity{ID: s.ID, OrderReference: s.OrderReference, Status: s.Status, CreatedBy: s.CreatedBy, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt, CompletedAt: s.CompletedAt}
}

func ToTransactionEntity(s TransactionStorage) domain.TransactionEntity {
	return domain.TransactionEntity{ID: s.ID, ReceivingDocumentID: s.ReceivingDocumentID, Status: s.Status, ReviewStatus: s.ReviewStatus, RejectionReason: s.RejectionReason, ReviewNote: s.ReviewNote, CreatedBy: s.CreatedBy, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt, ApprovedBy: s.ApprovedBy, ApprovedAt: s.ApprovedAt, CorrectedBy: s.CorrectedBy, CorrectedAt: s.CorrectedAt, ReviewedBy: s.ReviewedBy, ReviewedAt: s.ReviewedAt}
}

func ToTransactionStorage(e *domain.TransactionEntity) TransactionStorage {
	return TransactionStorage{ID: e.ID, ReceivingDocumentID: e.ReceivingDocumentID, Status: e.Status, ReviewStatus: e.ReviewStatus, RejectionReason: e.RejectionReason, ReviewNote: e.ReviewNote, CreatedBy: e.CreatedBy, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, ApprovedBy: e.ApprovedBy, ApprovedAt: e.ApprovedAt, CorrectedBy: e.CorrectedBy, CorrectedAt: e.CorrectedAt, ReviewedBy: e.ReviewedBy, ReviewedAt: e.ReviewedAt}
}
