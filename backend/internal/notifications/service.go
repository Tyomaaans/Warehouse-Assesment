package notifications

import (
	"context"
	"encoding/json"
	"strings"

	"warehouse/internal/notifications/domain"
	"warehouse/pkg"
)

type NotificationService interface {
	GetNotifications(context.Context, int, int) (*PaginatedNotificationResponse, error)
	MarkRead(context.Context, string) (*NotificationResponse, error)
}

type notificationService struct{ notificationRepo domain.NotificationRepository }

func NewNotificationService(repo domain.NotificationRepository) NotificationService {
	return &notificationService{notificationRepo: repo}
}

func (s *notificationService) GetNotifications(ctx context.Context, page, limit int) (*PaginatedNotificationResponse, error) {
	actor, err := pkg.RequireActor(ctx, "")
	if err != nil {
		return nil, err
	}
	if err := pkg.ValidatePagination(page, limit); err != nil {
		return nil, err
	}
	rows, total, err := s.notificationRepo.GetNotifications(ctx, actor.ID, page, limit)
	if err != nil {
		return nil, err
	}
	result := &PaginatedNotificationResponse{Notifications: make([]NotificationResponse, len(rows)), Meta: pkg.NewMeta(page, limit, total)}
	for i, row := range rows {
		result.Notifications[i] = ToNotificationResponse(row)
	}
	return result, nil
}

func (s *notificationService) MarkRead(ctx context.Context, rawID string) (*NotificationResponse, error) {
	actor, err := pkg.RequireActor(ctx, "")
	if err != nil {
		return nil, err
	}
	id, err := pkg.ParseID(rawID)
	if err != nil {
		return nil, err
	}
	row, err := s.notificationRepo.MarkRead(ctx, actor.ID, id)
	if err != nil {
		return nil, err
	}
	response := ToNotificationResponse(*row)
	return &response, nil
}

var eventTypes = []string{"TRANSACTION_SUBMITTED", "TRANSACTION_REJECTED", "TRANSACTION_CORRECTED", "REVIEW_RESOLVED", "TRANSACTION_APPROVED"}

func decodeEvent(body []byte, messageID string) (domain.Event, error) {
	var event domain.Event
	if json.Unmarshal(body, &event) != nil || event.SourceEventID != messageID || strings.TrimSpace(event.Message) == "" || len(event.Recipients) == 0 {
		return event, domain.ErrInvalidEvent
	}
	known := false
	for _, kind := range eventTypes {
		if kind == event.EventType {
			known = true
		}
	}
	if !known {
		return event, domain.ErrInvalidEvent
	}
	ids := append([]string{event.SourceEventID, event.ReceivingTransactionID, event.ReceivingDocumentID}, event.Recipients...)
	for _, raw := range ids {
		if _, err := pkg.ParseID(raw); err != nil {
			return event, domain.ErrInvalidEvent
		}
	}
	return event, nil
}
