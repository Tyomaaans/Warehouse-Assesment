package notifications

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"warehouse/internal/notifications/domain"
	"warehouse/pkg"
)

type outboxRepository struct{ db *gorm.DB }

func NewOutboxRepository(db *gorm.DB) domain.OutboxRepository { return &outboxRepository{db: db} }

func (r *outboxRepository) Claim(ctx context.Context, lease time.Duration) (*domain.OutboxEntity, error) {
	var row domain.OutboxEntity
	// One short autocommitted statement; no SQL transaction spans broker I/O.
	result := r.db.WithContext(ctx).Raw(`WITH candidate AS (
SELECT id FROM notification_outbox WHERE published_at IS NULL AND next_attempt_at<=clock_timestamp()
AND (lease_until IS NULL OR lease_until<=clock_timestamp())
ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1)
UPDATE notification_outbox o SET lease_token=?, lease_until=clock_timestamp()+(? * interval '1 millisecond'), attempts=attempts+1
FROM candidate c WHERE o.id=c.id RETURNING o.*`, uuid.New(), lease.Milliseconds()).Scan(&row)
	if result.Error != nil {
		return nil, pkg.HandleDBError(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &row, nil
}

func (r *outboxRepository) Published(ctx context.Context, event *domain.OutboxEntity) error {
	result := r.db.WithContext(ctx).Exec(`UPDATE notification_outbox SET published_at=clock_timestamp(),lease_token=NULL,lease_until=NULL
WHERE id=? AND lease_token=? AND published_at IS NULL`, event.ID, event.LeaseToken)
	if result.Error != nil {
		return pkg.HandleDBError(result.Error)
	}
	if result.RowsAffected != 1 {
		return pkg.ErrConflict
	}
	return nil
}

func (r *outboxRepository) Retry(ctx context.Context, event *domain.OutboxEntity, delay time.Duration) error {
	return pkg.HandleDBError(r.db.WithContext(ctx).Exec(`UPDATE notification_outbox
SET lease_token=NULL,lease_until=NULL,next_attempt_at=clock_timestamp()+(? * interval '1 millisecond')
WHERE id=? AND lease_token=? AND published_at IS NULL`, delay.Milliseconds(), event.ID, event.LeaseToken).Error)
}
