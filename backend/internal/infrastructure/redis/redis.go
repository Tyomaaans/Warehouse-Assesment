package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type clientLogger struct {
	logger zerolog.Logger
}

func NewRedisClient(ctx context.Context, rawURL string, logger zerolog.Logger) (*redis.Client, error) {
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL")
	}
	options.PoolSize, options.MinIdleConns = 10, 2
	options.DialTimeout, options.ReadTimeout, options.WriteTimeout = 3*time.Second, 3*time.Second, 3*time.Second
	options.ContextTimeoutEnabled = true
	options.PoolTimeout = 3 * time.Second
	redis.SetLogger(clientLogger{logger: logger})
	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("Redis unavailable")
	}
	return client, nil
}

func (l clientLogger) Printf(_ context.Context, _ string, _ ...interface{}) {
	l.logger.Warn().Str("event", "redis_client_error").Msg("Redis reported a dependency error")
}
