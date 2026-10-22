package pkg

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestT07CanonicalUUIDContract(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	encoded := EncodeID(id)
	if encoded != "VQ6EAOKbQdSnFkRmVUQAAA" {
		t.Fatalf("encoded ID = %q", encoded)
	}
	decoded, err := ParseID(encoded)
	if err != nil || decoded != id {
		t.Fatalf("ParseID() = %v, %v", decoded, err)
	}
	wrongVariant := id
	wrongVariant[8] = 0
	wrongVersion := id
	wrongVersion[6] = 0x10
	invalid := []string{
		"", id.String(), encoded + "==", " " + encoded, encoded[:21] + "B",
		"+" + encoded[1:], "/" + encoded[1:],
		base64.RawURLEncoding.EncodeToString(wrongVariant[:]),
		base64.RawURLEncoding.EncodeToString(wrongVersion[:]),
		base64.RawURLEncoding.EncodeToString(uuid.Nil[:]),
	}
	for _, value := range invalid {
		if _, err := ParseID(value); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("ParseID(%q) error = %v, want invalid input", value, err)
		}
	}
}

type validationRequest struct {
	Items []validationItem `json:"items" validate:"required,min=1,dive"`
}

type validationItem struct {
	ID       string `json:"id" validate:"required,base64uuid4"`
	Quantity int64  `json:"quantity" validate:"gt=0"`
}

func TestT07StrictRequestAndErrorContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validate := NewValidator()
	r := gin.New()
	r.POST("/validate", func(c *gin.Context) {
		var req validationRequest
		if err := BindJSON(c, &req); err != nil {
			HTTPError(c, err)
			return
		}
		if err := ValidateStruct(validate, req); err != nil {
			HTTPError(c, err)
			return
		}
		SuccessResponse(c, http.StatusOK, "valid", req)
	})
	valid := `{"items":[{"id":"VQ6EAOKbQdSnFkRmVUQAAA","quantity":1}]}`
	tests := []struct {
		name    string
		body    string
		status  int
		message string
	}{
		{name: "valid", body: valid, status: 200},
		{name: "unknown field", body: `{"items":[],"actor_id":"ignored"}`, status: 400},
		{name: "nested unknown field", body: strings.Replace(valid, `"quantity":1`, `"quantity":1,"actor_id":"ignored"`, 1), status: 400},
		{name: "null root", body: "null", status: 400},
		{name: "array root", body: "[]", status: 400},
		{name: "trailing value", body: valid + " {}", status: 400},
		{name: "invalid UTF8", body: "{\"items\":[],\"\xff\":1}", status: 400},
		{name: "non JSON whitespace", body: "\u00a0" + valid, status: 400},
		{name: "invalid ID", body: strings.Replace(valid, "VQ6EAOKbQdSnFkRmVUQAAA", "not-an-id", 1), status: 400, message: "items[0].id"},
		{name: "negative quantity", body: strings.Replace(valid, `"quantity":1`, `"quantity":-1`, 1), status: 400, message: "items[0].quantity must be a positive integer"},
		{name: "fractional quantity", body: strings.Replace(valid, `"quantity":1`, `"quantity":1.5`, 1), status: 400},
		{name: "overflow quantity", body: strings.Replace(valid, `"quantity":1`, `"quantity":9223372036854775808`, 1), status: 400},
		{name: "body limit", body: `{"items":[],"padding":"` + strings.Repeat("x", 1<<20) + `"}`, status: 413},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(response, request)
			if response.Code != tt.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, tt.status, response.Body.String())
			}
			var body struct {
				Success bool         `json:"success"`
				Error   *ErrorDetail `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if tt.status == 200 {
				if !body.Success || body.Error != nil {
					t.Fatalf("success envelope = %s", response.Body.String())
				}
				return
			}
			if body.Success || body.Error == nil || body.Error.Message == "" {
				t.Fatalf("error envelope = %s", response.Body.String())
			}
			if tt.status == 400 && body.Error.Code != "invalid_input" {
				t.Fatalf("error code = %q", body.Error.Code)
			}
			if tt.status == 413 && body.Error.Code != "payload_too_large" {
				t.Fatalf("error code = %q", body.Error.Code)
			}
			if !strings.Contains(body.Error.Message, tt.message) {
				t.Fatalf("message = %q, want %q", body.Error.Message, tt.message)
			}
		})
	}
}

func TestT07PublicErrorStatusContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{err: ErrUnauthorized, status: 401, code: "unauthorized"},
		{err: ErrForbidden, status: 403, code: "forbidden"},
		{err: ErrNotFound, status: 404, code: "not_found"},
		{err: ErrConflict, status: 409, code: "conflict"},
		{err: ErrAlreadyExists, status: 409, code: "already_exists"},
		{err: fmt.Errorf("%w: private connection details", ErrUnavailable), status: 503, code: "unavailable"},
		{err: errors.New("private SQL and credentials"), status: 500, code: "internal"},
	}
	for _, tt := range tests {
		response := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(response)
		HTTPError(c, tt.err)
		var body struct {
			Success bool        `json:"success"`
			Error   ErrorDetail `json:"error"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != tt.status || body.Success || body.Error.Code != tt.code || strings.Contains(body.Error.Message, "private") {
			t.Fatalf("HTTPError(%v) = %d %s", tt.err, response.Code, response.Body.String())
		}
	}
}
