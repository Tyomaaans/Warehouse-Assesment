package config

import (
	"testing"
	"time"
)

func TestAuthConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://warehouse:password@localhost/warehouse?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("RABBITMQ_URL", "amqp://warehouse:password@localhost:5672/")
	t.Setenv("TRUSTED_PROXIES", "")
	t.Setenv("SHORT_REFRESH_EXPIRY", "")
	t.Setenv("DEFAULT_REFRESH_EXPIRY", "")
	t.Setenv("JWT_SECRET_KEY", "")
	cfg, err := NewConfig()
	if err != nil || cfg.ShortRefreshExpiry != time.Hour || cfg.DefaultRefreshExpiry != 168*time.Hour {
		t.Fatalf("NewConfig() refresh defaults error = %v", err)
	}
	if err := cfg.ValidateServer(); err == nil {
		t.Fatal("serving without a signing secret must fail")
	}
	t.Setenv("JWT_SECRET_KEY", "task2b-config-test-secret-at-least-32-bytes")
	cfg, err = NewConfig()
	if err != nil || cfg.ValidateServer() != nil {
		t.Fatalf("valid auth configuration error = %v", err)
	}
	for _, expiry := range []string{"0s", "-1h", "1.5s", "invalid"} {
		t.Setenv("SHORT_REFRESH_EXPIRY", expiry)
		if _, err := NewConfig(); err == nil {
			t.Fatalf("invalid SHORT_REFRESH_EXPIRY %q accepted", expiry)
		}
	}
}
