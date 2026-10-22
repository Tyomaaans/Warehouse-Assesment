package inventory

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"warehouse/pkg"
)

type InventoryHandler struct {
	inventorySvc InventoryService
}

func NewInventoryHandler(inventorySvc InventoryService) *InventoryHandler {
	return &InventoryHandler{inventorySvc: inventorySvc}
}

func (h *InventoryHandler) GetBalances(c *gin.Context) {
	page, limit, err := pkg.ParsePagination(c)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	result, err := h.inventorySvc.GetBalances(c.Request.Context(), c.Query("search"), page, limit)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 200, "inbound inventory balances retrieved", result)
}

func (h *InventoryHandler) GetMovements(c *gin.Context) {
	rawID := c.Query("product_id")
	if _, supplied := c.Request.URL.Query()["product_id"]; supplied && rawID == "" {
		pkg.HTTPError(c, fmt.Errorf("%w: product_id must be a canonical Base64URL UUID v4", pkg.ErrInvalidInput))
		return
	}
	page, limit, err := pkg.ParsePagination(c)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	result, err := h.inventorySvc.GetMovements(c.Request.Context(), rawID, page, limit)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 200, "inbound inventory history retrieved", result)
}
