package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidEvent = errors.New("invalid notification event")

type NotificationEntity struct {
	ID                     uuid.UUID
	RecipientID            uuid.UUID
	SourceEventID          uuid.UUID
	EventType              string
	ReceivingTransactionID *uuid.UUID
	Message                string
	ReadAt                 *time.Time
	CreatedAt              time.Time
}

// Event is the existing transactional-outbox payload, unchanged on the wire.
type Event struct {
	SourceEventID          string   `json:"source_event_id"`
	ReceivingTransactionID string   `json:"receiving_transaction_id"`
	ReceivingDocumentID    string   `json:"receiving_document_id"`
	EventType              string   `json:"event_type"`
	Recipients             []string `json:"recipients"`
	Message                string   `json:"message"`
}

type OutboxEntity struct {
	ID         uuid.UUID
	EventType  string
	Payload    json.RawMessage
	CreatedAt  time.Time
	LeaseToken uuid.UUID
	Attempts   int64
}

type NotificationRepository interface {
	GetNotifications(context.Context, uuid.UUID, int, int) ([]NotificationEntity, int64, error)
	MarkRead(context.Context, uuid.UUID, uuid.UUID) (*NotificationEntity, error)
	PersistEvent(context.Context, Event, []byte) error
}

type OutboxRepository interface {
	Claim(context.Context, time.Duration) (*OutboxEntity, error)
	Published(context.Context, *OutboxEntity) error
	Retry(context.Context, *OutboxEntity, time.Duration) error
}
