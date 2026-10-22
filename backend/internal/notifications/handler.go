package notifications

import (
	"github.com/gin-gonic/gin"

	"warehouse/pkg"
)

type NotificationHandler struct{ notificationSvc NotificationService }

func NewNotificationHandler(service NotificationService) *NotificationHandler {
	return &NotificationHandler{notificationSvc: service}
}

func (h *NotificationHandler) GetNotifications(c *gin.Context) {
	if _, err := pkg.RequireActor(c.Request.Context(), ""); err != nil {
		pkg.HTTPError(c, err)
		return
	}
	page, limit, err := pkg.ParsePagination(c)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	result, err := h.notificationSvc.GetNotifications(c.Request.Context(), page, limit)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 200, "notifications retrieved", result)
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	if _, err := pkg.RequireActor(c.Request.Context(), ""); err != nil {
		pkg.HTTPError(c, err)
		return
	}
	result, err := h.notificationSvc.MarkRead(c.Request.Context(), c.Param("id"))
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 200, "notification marked as read", result)
}
