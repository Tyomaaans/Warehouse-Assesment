package notifications

import (
	"time"

	"warehouse/pkg"
)

type NotificationResponse struct {
	ID                     string     `json:"id"`
	RecipientID            string     `json:"recipient_id"`
	SourceEventID          string     `json:"source_event_id"`
	EventType              string     `json:"event_type"`
	ReceivingTransactionID *string    `json:"receiving_transaction_id"`
	Message                string     `json:"message"`
	ReadAt                 *time.Time `json:"read_at"`
	CreatedAt              time.Time  `json:"created_at"`
}

type PaginatedNotificationResponse struct {
	Notifications []NotificationResponse `json:"notifications"`
	Meta          pkg.PaginationMeta     `json:"meta"`
}
