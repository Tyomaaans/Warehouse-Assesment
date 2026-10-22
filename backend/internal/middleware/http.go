package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"warehouse/pkg"
)

func RequestContext(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		id := pkg.EncodeID(uuid.New())
		c.Header("X-Request-ID", id)
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error().Str("event", "request_panic").Str("request_id", id).Msg("request panicked")
				if !c.Writer.Written() {
					pkg.HTTPError(c, pkg.ErrInternal)
				} else {
					c.Abort()
				}
			}
			route := c.FullPath()
			if route == "" {
				route = "unmatched"
			}
			logger.Info().Str("event", "request_completed").Str("request_id", id).
				Str("peer_address", c.Request.RemoteAddr).Str("client_ip", c.ClientIP()).
				Str("method", c.Request.Method).Str("route", route).
				Int("status", c.Writer.Status()).Dur("duration_ms", time.Since(started)).Msg("request completed")
		}()
		c.Next()
	}
}
