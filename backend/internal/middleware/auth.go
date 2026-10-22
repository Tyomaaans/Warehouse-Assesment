package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"warehouse/internal/auth/token"
	"warehouse/pkg"
)

type AuthMiddleware struct {
	tokenSvc token.JWTService
}

func NewAuthMiddleware(tokenSvc token.JWTService) *AuthMiddleware {
	return &AuthMiddleware{tokenSvc: tokenSvc}
}

func (m *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		headers := c.Request.Header.Values("Authorization")
		if len(headers) != 1 {
			pkg.HTTPError(c, pkg.ErrUnauthorized)
			return
		}
		parts := strings.Fields(headers[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			pkg.HTTPError(c, pkg.ErrUnauthorized)
			return
		}
		actor, err := m.tokenSvc.ValidateAccessToken(c.Request.Context(), parts[1])
		if err != nil {
			pkg.HTTPError(c, err)
			return
		}
		c.Set("userID", actor.ID.String())
		c.Set("sessionID", actor.SessionID.String())
		c.Set("role", actor.Role)
		c.Request = c.Request.WithContext(pkg.WithActor(c.Request.Context(), actor))
		c.Next()
	}
}

func (m *AuthMiddleware) RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, err := pkg.RequireActor(c.Request.Context(), "")
		if err != nil {
			pkg.HTTPError(c, err)
			return
		}
		for _, role := range roles {
			if actor.Role == role {
				c.Next()
				return
			}
		}
		pkg.HTTPError(c, pkg.ErrForbidden)
	}
}
