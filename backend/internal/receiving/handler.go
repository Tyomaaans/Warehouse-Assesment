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

func (h *ReceivingHandler) CreateTransaction(c *gin.Context) {
	var req CreateTransactionRequest
	if err := pkg.BindJSON(c, &req); err != nil {
		pkg.HTTPError(c, err)
		return
	}
	result, err := h.receivingSvc.CreateTransaction(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 201, "receiving transaction recorded", result)
}

func (h *ReceivingHandler) GetTransactions(c *gin.Context) {
	page, limit, err := pkg.ParsePagination(c)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	result, err := h.receivingSvc.GetTransactions(c.Request.Context(), c.Param("id"), page, limit)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 200, "receiving transactions retrieved", result)
}

func (h *ReceivingHandler) GetTransaction(c *gin.Context) {
	result, err := h.receivingSvc.GetTransaction(c.Request.Context(), c.Param("id"))
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 200, "receiving transaction retrieved", result)
}

func (h *ReceivingHandler) CorrectTransaction(c *gin.Context) {
	var req CorrectTransactionRequest
	if err := pkg.BindJSON(c, &req); err != nil {
		pkg.HTTPError(c, err)
		return
	}
	result, err := h.receivingSvc.CorrectTransaction(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 200, "receiving transaction corrected", result)
}

func (h *ReceivingHandler) ReviewTransaction(c *gin.Context) {
	var req ReviewTransactionRequest
	if err := pkg.BindJSON(c, &req); err != nil {
		pkg.HTTPError(c, err)
		return
	}
	result, err := h.receivingSvc.ReviewTransaction(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 200, "rejected transaction review resolved", result)
}

func (h *ReceivingHandler) ApproveTransaction(c *gin.Context) {
	result, err := h.receivingSvc.ApproveTransaction(c.Request.Context(), c.Param("id"))
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 200, "receiving transaction approved", result)
}

func (h *ReceivingHandler) GetAuditLogs(c *gin.Context) {
	rawID := c.Query("receiving_id")
	if _, supplied := c.Request.URL.Query()["receiving_id"]; supplied && rawID == "" {
		pkg.HTTPError(c, fmt.Errorf("%w: receiving_id must be a canonical Base64URL UUID v4", pkg.ErrInvalidInput))
		return
	}
	page, limit, err := pkg.ParsePagination(c)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	result, err := h.receivingSvc.GetAuditLogs(c.Request.Context(), rawID, page, limit)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 200, "receiving audit history retrieved", result)
}
