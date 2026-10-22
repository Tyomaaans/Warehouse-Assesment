package token

import (
	"context"
	"errors"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"warehouse/pkg"
)

func TestInvalidAccessTokensFailBeforeSessionLookup(t *testing.T) {
	secret := "unit-test-signing-secret-at-least-32-bytes"
	service := NewJWTService(secret, 168*time.Hour, time.Hour, nil, nil, nil)
	now := time.Now()
	for _, name := range []string{"expired", "missing_exp", "missing_iat", "future_iat", "wrong_algorithm", "wrong_signature", "bad_user", "bad_session", "bad_role", "wrong_lifetime"} {
		t.Run(name, func(t *testing.T) {
			claims := &jwtClaims{SessionID: uuid.NewString(), Role: pkg.Operator, RegisteredClaims: gojwt.RegisteredClaims{
				Subject: uuid.NewString(), IssuedAt: gojwt.NewNumericDate(now), ExpiresAt: gojwt.NewNumericDate(now.Add(AccessTokenExpiry)),
			}}
			method, key := gojwt.SigningMethodHS256, secret
			switch name {
			case "expired":
				claims.IssuedAt, claims.ExpiresAt = gojwt.NewNumericDate(now.Add(-time.Hour)), gojwt.NewNumericDate(now.Add(-45*time.Minute))
			case "missing_exp":
				claims.ExpiresAt = nil
			case "missing_iat":
				claims.IssuedAt = nil
			case "future_iat":
				claims.IssuedAt, claims.ExpiresAt = gojwt.NewNumericDate(now.Add(time.Hour)), gojwt.NewNumericDate(now.Add(time.Hour+AccessTokenExpiry))
			case "wrong_algorithm":
				method = gojwt.SigningMethodHS512
			case "wrong_signature":
				key = "different-signing-secret-at-least-32-bytes"
			case "bad_user":
				claims.Subject = pkg.EncodeID(uuid.New())
			case "bad_session":
				claims.SessionID = uuid.NewString() + " "
			case "bad_role":
				claims.Role = "ADMIN"
			case "wrong_lifetime":
				claims.ExpiresAt = gojwt.NewNumericDate(now.Add(time.Hour))
			}
			raw, err := gojwt.NewWithClaims(method, claims).SignedString([]byte(key))
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.ValidateAccessToken(context.Background(), raw)
			if !errors.Is(err, pkg.ErrUnauthorized) {
				t.Fatalf("ValidateAccessToken() error = %v, want unauthorized", err)
			}
		})
	}
}
