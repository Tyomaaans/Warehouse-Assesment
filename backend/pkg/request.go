package pkg

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

func EncodeID(id uuid.UUID) string { return base64.RawURLEncoding.EncodeToString(id[:]) }

func ParseID(value string) (uuid.UUID, error) {
	if len(value) != 22 {
		return uuid.Nil, fmt.Errorf("%w: ID must be a canonical Base64URL UUID v4", ErrInvalidInput)
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(data) != 16 {
		return uuid.Nil, fmt.Errorf("%w: ID must be a canonical Base64URL UUID v4", ErrInvalidInput)
	}
	id, err := uuid.FromBytes(data)
	if err != nil || id.Version() != 4 || id.Variant() != uuid.RFC4122 || EncodeID(id) != value {
		return uuid.Nil, fmt.Errorf("%w: ID must be a canonical Base64URL UUID v4", ErrInvalidInput)
	}
	return id, nil
}

func EncodeNullableID(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	value := EncodeID(*id)
	return &value
}

func BindJSON(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			return ErrPayloadTooLarge
		}
		return ErrInvalidInput
	}
	data = bytes.Trim(data, " \t\r\n")
	if !utf8.Valid(data) || len(data) == 0 || data[0] != '{' {
		return fmt.Errorf("%w: body must be a JSON object", ErrInvalidInput)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: malformed JSON, unknown field or invalid field type", ErrInvalidInput)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("%w: body must contain one JSON object", ErrInvalidInput)
	}
	return nil
}

func NewValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(field reflect.StructField) string { return strings.Split(field.Tag.Get("json"), ",")[0] })
	_ = v.RegisterValidation("base64uuid4", func(fl validator.FieldLevel) bool { _, err := ParseID(fl.Field().String()); return err == nil })
	_ = v.RegisterValidation("notblank", func(fl validator.FieldLevel) bool {
		return strings.TrimSpace(fl.Field().String()) != "" && !strings.ContainsRune(fl.Field().String(), 0)
	})
	return v
}

func ValidateStruct(v *validator.Validate, value any) error {
	if err := v.Struct(value); err != nil {
		var fields validator.ValidationErrors
		if !errors.As(err, &fields) {
			return ErrInvalidInput
		}
		messages := make([]string, 0, len(fields))
		for _, field := range fields {
			name := field.Namespace()
			if dot := strings.IndexByte(name, '.'); dot >= 0 {
				name = name[dot+1:]
			}
			message := name + " is invalid"
			switch field.Tag() {
			case "required", "notblank":
				message = name + " is required and cannot be blank"
			case "gt":
				message = name + " must be a positive integer"
			case "base64uuid4":
				message = name + " must be a canonical Base64URL UUID v4"
			case "email":
				message = name + " must be a valid email"
			case "min":
				message = name + " must contain at least " + field.Param() + " item(s)"
			}
			messages = append(messages, message)
		}
		return fmt.Errorf("%w: %s", ErrInvalidInput, strings.Join(messages, ", "))
	}
	return nil
}

type PaginationMeta struct {
	CurrentPage int   `json:"current_page"`
	TotalPages  int64 `json:"total_pages"`
	PageSize    int   `json:"page_size"`
	TotalData   int64 `json:"total_data"`
}

func NewMeta(page, limit int, total int64) PaginationMeta {
	return PaginationMeta{CurrentPage: page, TotalPages: (total + int64(limit) - 1) / int64(limit), PageSize: limit, TotalData: total}
}

func ValidatePagination(page, limit int) error {
	if page < 1 || page > 10000 || limit < 1 || limit > 100 {
		return fmt.Errorf("%w: page must be 1–10000 and limit must be 1–100", ErrInvalidInput)
	}
	return nil
}

func ParsePagination(c *gin.Context) (int, int, error) {
	values := []string{c.DefaultQuery("page", "1"), c.DefaultQuery("limit", "10")}
	result := make([]int, 2)
	for i, value := range values {
		if value == "" || strings.Trim(value, "0123456789") != "" {
			return 0, 0, fmt.Errorf("%w: invalid pagination", ErrInvalidInput)
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return 0, 0, fmt.Errorf("%w: invalid pagination", ErrInvalidInput)
		}
		result[i] = n
	}
	if err := ValidatePagination(result[0], result[1]); err != nil {
		return 0, 0, err
	}
	return result[0], result[1], nil
}

func ValidateSearch(value string) error {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 100 || strings.ContainsRune(value, 0) {
		return fmt.Errorf("%w: search must be at most 100 characters", ErrInvalidInput)
	}
	return nil
}

func SearchPattern(value string) string {
	value = strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value)
	return "%" + value + "%"
}
