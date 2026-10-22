package pkg

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrNotFound           = errors.New("record not found")
	ErrAlreadyExists      = errors.New("record already exists")
	ErrConflict           = errors.New("operation conflicts with current state")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthorized       = errors.New("authentication required")
	ErrForbidden          = errors.New("access denied")
	ErrConcurrentRefresh  = errors.New("refresh token rotation is currently processing")
	ErrUnavailable        = errors.New("service temporarily unavailable")
	ErrInternal           = errors.New("internal server error")
	ErrPayloadTooLarge    = errors.New("request body exceeds 1 MiB")
)

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Error   any    `json:"error,omitempty"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func SuccessResponse(c *gin.Context, status int, message string, data any) {
	c.JSON(status, Response{Success: true, Message: message, Data: data})
}

func HTTPError(c *gin.Context, err error) {
	status, code, message := http.StatusInternalServerError, "internal", ErrInternal.Error()
	switch {
	case errors.Is(err, ErrInvalidInput):
		status, code, message = 400, "invalid_input", err.Error()
	case errors.Is(err, ErrNotFound):
		status, code, message = 404, "not_found", ErrNotFound.Error()
	case errors.Is(err, ErrAlreadyExists):
		status, code, message = 409, "already_exists", ErrAlreadyExists.Error()
	case errors.Is(err, ErrConflict):
		status, code, message = 409, "conflict", err.Error()
	case errors.Is(err, ErrInvalidCredentials):
		status, code, message = 401, "invalid_credentials", ErrInvalidCredentials.Error()
	case errors.Is(err, ErrUnauthorized):
		status, code, message = 401, "unauthorized", ErrUnauthorized.Error()
	case errors.Is(err, ErrForbidden):
		status, code, message = 403, "forbidden", ErrForbidden.Error()
	case errors.Is(err, ErrConcurrentRefresh):
		status, code, message = 409, "concurrent_refresh", ErrConcurrentRefresh.Error()
	case errors.Is(err, ErrPayloadTooLarge):
		status, code, message = 413, "payload_too_large", ErrPayloadTooLarge.Error()
	case errors.Is(err, ErrUnavailable), errors.Is(err, context.DeadlineExceeded):
		status, code, message = 503, "unavailable", ErrUnavailable.Error()
	}
	if status >= 500 {
		_ = c.Error(err)
	}
	c.AbortWithStatusJSON(status, Response{Success: false, Error: ErrorDetail{Code: code, Message: message}})
}

func HandleDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	for _, known := range []error{ErrInvalidInput, ErrNotFound, ErrAlreadyExists, ErrConflict, ErrForbidden, ErrUnauthorized, ErrUnavailable} {
		if errors.Is(err, known) {
			return err
		}
	}
	var networkError net.Error
	if errors.Is(err, driver.ErrBadConn) || errors.As(err, &networkError) || errors.Is(err, context.DeadlineExceeded) {
		return ErrUnavailable
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrAlreadyExists
		case "23503":
			return fmt.Errorf("%w: referenced item does not exist", ErrInvalidInput)
		case "23502", "23514", "22P02", "22003", "22001":
			return fmt.Errorf("%w: values do not satisfy the allowed format or range", ErrInvalidInput)
		case "40001", "40P01", "55P03":
			return ErrConflict
		case "08000", "08003", "08006", "53300", "57P01":
			return ErrUnavailable
		}
	}
	return fmt.Errorf("%w: database operation failed: %w", ErrInternal, err)
}
