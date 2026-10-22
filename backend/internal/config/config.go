package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultPort = "8080"

type AppConfig struct {
	AppPort              string
	Environment          string
	DatabaseURL          string
	RedisURL             string
	RabbitMQURL          string
	JWTSecretKey         string
	DefaultRefreshExpiry time.Duration
	ShortRefreshExpiry   time.Duration
	TrustedProxies       []string
	StartupTimeout       time.Duration
	ShutdownTimeout      time.Duration
}

func NewConfig() (AppConfig, error) {
	cfg := AppConfig{
		AppPort:         env("APP_PORT", DefaultPort),
		Environment:     env("APP_ENV", "development"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisURL:        os.Getenv("REDIS_URL"),
		RabbitMQURL:     os.Getenv("RABBITMQ_URL"),
		JWTSecretKey:    os.Getenv("JWT_SECRET_KEY"),
		StartupTimeout:  30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}
	if cfg.Environment != "development" && cfg.Environment != "test" && cfg.Environment != "production" {
		return cfg, fmt.Errorf("invalid APP_ENV")
	}
	if port, err := strconv.ParseUint(cfg.AppPort, 10, 16); err != nil || port == 0 {
		return cfg, fmt.Errorf("invalid APP_PORT")
	}
	if err := validateURL("DATABASE_URL", cfg.DatabaseURL, "postgres", "postgresql"); err != nil {
		return cfg, err
	}
	if cfg.RedisURL != "" {
		if err := validateURL("REDIS_URL", cfg.RedisURL, "redis", "rediss"); err != nil {
			return cfg, err
		}
	}
	if cfg.RabbitMQURL != "" {
		if err := validateURL("RABBITMQ_URL", cfg.RabbitMQURL, "amqp", "amqps"); err != nil {
			return cfg, err
		}
	}
	if proxies := os.Getenv("TRUSTED_PROXIES"); proxies != "" {
		for _, value := range strings.Split(proxies, ",") {
			value = strings.TrimSpace(value)
			if net.ParseIP(value) == nil {
				if _, _, err := net.ParseCIDR(value); err != nil {
					return cfg, fmt.Errorf("invalid TRUSTED_PROXIES")
				}
			}
			cfg.TrustedProxies = append(cfg.TrustedProxies, value)
		}
	}
	var err error
	cfg.DefaultRefreshExpiry, err = refreshDuration("DEFAULT_REFRESH_EXPIRY", "168h")
	if err != nil {
		return cfg, err
	}
	cfg.ShortRefreshExpiry, err = refreshDuration("SHORT_REFRESH_EXPIRY", "1h")
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c AppConfig) ValidateServer() error {
	if c.RedisURL == "" || c.RabbitMQURL == "" {
		return fmt.Errorf("REDIS_URL and RABBITMQ_URL are required for serving")
	}
	if len(c.JWTSecretKey) < 32 || strings.TrimSpace(c.JWTSecretKey) != c.JWTSecretKey {
		return fmt.Errorf("JWT_SECRET_KEY must contain at least 32 bytes without surrounding whitespace")
	}
	return nil
}

func refreshDuration(name, fallback string) (time.Duration, error) {
	duration, err := time.ParseDuration(env(name, fallback))
	if err != nil || duration < time.Second || duration%time.Second != 0 {
		return 0, fmt.Errorf("%s must be a positive duration in whole seconds", name)
	}
	return duration, nil
}

func validateURL(name, raw string, schemes ...string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.Fragment != "" {
		return fmt.Errorf("invalid %s", name)
	}
	for _, scheme := range schemes {
		if u.Scheme == scheme {
			return nil
		}
	}
	return fmt.Errorf("invalid %s", name)
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
