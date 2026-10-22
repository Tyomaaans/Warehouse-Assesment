package notifications

import (
	"warehouse/internal/notifications/domain"
	"warehouse/pkg"
)

func ToNotificationResponse(e domain.NotificationEntity) NotificationResponse {
	return NotificationResponse{
		ID: pkg.EncodeID(e.ID), RecipientID: pkg.EncodeID(e.RecipientID), SourceEventID: pkg.EncodeID(e.SourceEventID),
		EventType: e.EventType, ReceivingTransactionID: pkg.EncodeNullableID(e.ReceivingTransactionID),
		Message: e.Message, ReadAt: e.ReadAt, CreatedAt: e.CreatedAt,
	}
}
