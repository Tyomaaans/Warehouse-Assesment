package products

import (
	"github.com/gin-gonic/gin"

	"warehouse/pkg"
)

type ProductHandler struct{ productSvc ProductService }

func NewProductHandler(svc ProductService) *ProductHandler { return &ProductHandler{productSvc: svc} }

func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var req CreateProductRequest
	if err := pkg.BindJSON(c, &req); err != nil {
		pkg.HTTPError(c, err)
		return
	}
	result, err := h.productSvc.CreateProduct(c.Request.Context(), req)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 201, "product created successfully", result)
}

func (h *ProductHandler) GetProducts(c *gin.Context) {
	page, limit, err := pkg.ParsePagination(c)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	result, err := h.productSvc.GetProducts(c.Request.Context(), c.Query("search"), page, limit)
	if err != nil {
		pkg.HTTPError(c, err)
		return
	}
	pkg.SuccessResponse(c, 200, "products retrieved successfully", result)
}
