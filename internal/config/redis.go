package config

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis client tuning constants.
//
// Pool sizing (PoolSize, MinIdleConns) lives in Config because it may vary
// between environments. Timeouts and retry behavior are kept as constants
// because they rarely change per-environment.
const (
	redisDialTimeout     = 5 * time.Second
	redisReadTimeout     = 3 * time.Second
	redisWriteTimeout    = 3 * time.Second
	redisConnMaxIdleTime = 5 * time.Minute
	redisConnMaxLifetime = 1 * time.Hour
	redisMaxRetries      = 3
	redisMinRetryBackoff = 100 * time.Millisecond
	redisMaxRetryBackoff = 500 * time.Millisecond
	redisPingTimeout     = 5 * time.Second
)

// NewRedisClient creates a Redis client and verifies the connection with a
// ping. It returns an error on any failure so the caller can decide how to
// handle startup issues.
func NewRedisClient(ctx context.Context, cfg *Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,

		// Connection timeouts — fail fast instead of hanging.
		DialTimeout:  redisDialTimeout,
		ReadTimeout:  redisReadTimeout,
		WriteTimeout: redisWriteTimeout,

		// Pool tuning — sourced from .env via Config.
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,

		// Connection lifecycle — prevents stale and leaked connections.
		ConnMaxIdleTime: redisConnMaxIdleTime,
		ConnMaxLifetime: redisConnMaxLifetime,

		// Retry policy — handles transient failures (network blips, restarts).
		MaxRetries:      redisMaxRetries,
		MinRetryBackoff: redisMinRetryBackoff,
		MaxRetryBackoff: redisMaxRetryBackoff,
	})

	// Verify the client can actually reach Redis.
	pingCtx, cancel := context.WithTimeout(ctx, redisPingTimeout)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return client, nil
}