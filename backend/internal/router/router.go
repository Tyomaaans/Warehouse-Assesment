package router

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"warehouse/internal/auth/user"
	"warehouse/internal/middleware"
	"warehouse/internal/products"
	"warehouse/internal/receiving"
	"warehouse/pkg"
)

func NewRouter(logger zerolog.Logger, ready func(context.Context) error, trustedProxies []string, authHandler *user.UserAuthHandler, authMiddleware *middleware.AuthMiddleware, productHandler *products.ProductHandler, receivingHandler *receiving.ReceivingHandler) (*gin.Engine, error) {
	r := gin.New()
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		return nil, err
	}
	r.Use(middleware.RequestContext(logger))
	r.NoRoute(func(c *gin.Context) {
		pkg.HTTPError(c, pkg.ErrNotFound)
	})
	v1 := r.Group("/api/v1")
	auth := v1.Group("/auth")
	auth.POST("/login", authHandler.LoginUser)
	auth.POST("/refresh", authHandler.RefreshTokens)
	auth.POST("/logout", authMiddleware.Authenticate(), authHandler.LogoutUser)
	protected := v1.Group("", authMiddleware.Authenticate())
	protected.GET("/products", authMiddleware.RequireRole(pkg.Operator, pkg.Supervisor), productHandler.GetProducts)
	protected.POST("/products", authMiddleware.RequireRole(pkg.Supervisor), productHandler.CreateProduct)
	protected.GET("/receivings", authMiddleware.RequireRole(pkg.Operator, pkg.Supervisor), receivingHandler.GetDocuments)
	protected.POST("/receivings", authMiddleware.RequireRole(pkg.Operator), receivingHandler.CreateDocument)
	protected.GET("/receivings/:id", authMiddleware.RequireRole(pkg.Operator, pkg.Supervisor), receivingHandler.GetDocument)
	v1.GET("/health", func(c *gin.Context) {
		pkg.SuccessResponse(c, http.StatusOK, "healthy", gin.H{"status": "ok"})
	})
	v1.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		if err := ready(ctx); err != nil {
			pkg.HTTPError(c, pkg.ErrUnavailable)
			return
		}
		pkg.SuccessResponse(c, http.StatusOK, "ready", gin.H{"status": "ready"})
	})
	return r, nil
}
