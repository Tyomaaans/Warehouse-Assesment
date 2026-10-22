package receiving

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"warehouse/pkg"
)

type ReceivingHandler struct {
	receivingSvc ReceivingService
}

func NewReceivingHandler(receivingSvc ReceivingService) *ReceivingHandler {
	return &ReceivingHandler{receivingSvc: receivingSvc}
}

func (h *ReceivingHandler) CreateDocument(c *gin.Context) {
	var req CreateReceivingRequest
	if err := pkg.BindJSON(c, &req); err != nil {
		pkg.HTTPError(c, err)
		return
	}
	result, err := h.receivingSvc.CreateDocument(c.Request.Context(), req)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 201, "receiving document created successfully", result)
}

func (h *ReceivingHandler) GetDocuments(c *gin.Context) {
	status := c.Query("status")
	if _, supplied := c.Request.URL.Query()["status"]; supplied && status == "" {
		pkg.HTTPError(c, fmt.Errorf("%w: status must be INCOMPLETE or COMPLETED", pkg.ErrInvalidInput))
		return
	}
	page, limit, err := pkg.ParsePagination(c)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	result, err := h.receivingSvc.GetDocuments(c.Request.Context(), status, page, limit)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 200, "receiving documents retrieved successfully", result)
}

func (h *ReceivingHandler) GetDocument(c *gin.Context) {
	result, err := h.receivingSvc.GetDocument(c.Request.Context(), c.Param("id"))
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 200, "receiving document retrieved successfully", result)
}
