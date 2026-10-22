package notifications

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"warehouse/internal/notifications/domain"
	"warehouse/pkg"
)

type notificationRepository struct{ db *gorm.DB }

func NewNotificationRepository(db *gorm.DB) domain.NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) GetNotifications(ctx context.Context, recipient uuid.UUID, page, limit int) ([]domain.NotificationEntity, int64, error) {
	rows := make([]domain.NotificationEntity, 0)
	var total int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Table("notifications").Where("recipient_id=?", recipient)
		if err := query.Count(&total).Error; err != nil {
			return err
		}
		return query.Order("created_at DESC, id ASC").Limit(limit).Offset((page - 1) * limit).Scan(&rows).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return rows, total, pkg.HandleDBError(err)
}

func (r *notificationRepository) MarkRead(ctx context.Context, recipient, id uuid.UUID) (*domain.NotificationEntity, error) {
	var row domain.NotificationEntity
	result := r.db.WithContext(ctx).Raw(`UPDATE notifications SET read_at=COALESCE(read_at,clock_timestamp())
WHERE id=? AND recipient_id=? RETURNING *`, id, recipient).Scan(&row)
	if result.Error != nil {
		return nil, pkg.HandleDBError(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, pkg.ErrNotFound
	}
	return &row, nil
}

func (r *notificationRepository) PersistEvent(ctx context.Context, event domain.Event, body []byte) error {
	id, _ := pkg.ParseID(event.SourceEventID)
	transactionID, _ := pkg.ParseID(event.ReceivingTransactionID)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Only the exact committed source payload may determine recipients/message.
		var exists bool
		if err := tx.Raw(`SELECT EXISTS(SELECT 1 FROM notification_outbox
WHERE id=? AND aggregate_id=? AND event_type=? AND payload=?::jsonb)`, id, transactionID, event.EventType, string(body)).Scan(&exists).Error; err != nil {
			return err
		}
		if !exists {
			return domain.ErrInvalidEvent
		}
		for _, raw := range event.Recipients {
			recipient, _ := pkg.ParseID(raw)
			if err := tx.Exec(`INSERT INTO notifications(id,recipient_id,source_event_id,event_type,receiving_transaction_id,message,created_at)
VALUES (?,?,?,?,?,?,clock_timestamp()) ON CONFLICT(source_event_id,recipient_id) DO NOTHING`,
				uuid.New(), recipient, id, event.EventType, transactionID, event.Message).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err == domain.ErrInvalidEvent {
		return err
	}
	return pkg.HandleDBError(err)
}
