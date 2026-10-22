package receiving

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"warehouse/internal/receiving/domain"
	"warehouse/pkg"
)

func (s *receivingService) GetAuditLogs(ctx context.Context, rawID string, page, limit int) (*PaginatedAuditResponse, error) {
	if _, err := pkg.RequireActor(ctx, pkg.Supervisor); err != nil {
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
	list, total, err := s.receivingRepo.GetAuditLogs(ctx, id, page, limit)
	if err != nil {
		return nil, err
	}
	result := &PaginatedAuditResponse{AuditLogs: make([]AuditRecordResponse, len(list)), Meta: pkg.NewMeta(page, limit, total)}
	for i, row := range list {
		result.AuditLogs[i] = AuditRecordResponse{
			ID: pkg.EncodeID(row.ID), ReceivingDocumentID: pkg.EncodeID(row.ReceivingDocumentID),
			ReceivingTransactionID: pkg.EncodeNullableID(row.ReceivingTransactionID), ActorID: pkg.EncodeNullableID(row.ActorID),
			EventType: row.EventType, Reason: row.Reason, BeforeData: row.BeforeData, AfterData: row.AfterData, CreatedAt: row.CreatedAt,
		}
	}
	return result, nil
}

func (r *receivingRepository) GetAuditLogs(ctx context.Context, id *uuid.UUID, page, limit int) ([]domain.AuditRecordEntity, int64, error) {
	rows := make([]domain.AuditRecordEntity, 0)
	var total int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Table("receiving_audit_logs")
		if id != nil {
			query = query.Where("receiving_document_id = ?", *id)
		}
		if err := query.Count(&total).Error; err != nil {
			return err
		}
		return query.Order("created_at DESC, id ASC").Limit(limit).Offset((page - 1) * limit).Scan(&rows).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return rows, total, pkg.HandleDBError(err)
}
