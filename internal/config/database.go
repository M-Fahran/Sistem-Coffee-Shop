package config

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Database pool tuning constants.
//
// These values apply to all environments. If a specific deployment needs
// different values, promote them to Config and source them from .env.
const (
	dbMaxConnLifetime   = 1 * time.Hour
	dbMaxConnIdleTime   = 30 * time.Minute
	dbHealthCheckPeriod = 1 * time.Minute
	dbConnectTimeout    = 5 * time.Second
	dbPingTimeout       = 5 * time.Second
)

// NewPostgresPool creates a pgx connection pool and verifies the connection
// with a ping. It returns an error on any failure so the caller can decide
// how to handle startup issues (retry, fallback, fail-fast).
func NewPostgresPool(ctx context.Context, cfg *Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database DSN: %w", err)
	}

	// Pool tuning — sourced from .env via Config.
	poolConfig.MaxConns = cfg.Database.MaxConns
	poolConfig.MinConns = cfg.Database.MinConns

	// Connection lifecycle — prevents stale and leaked connections.
	poolConfig.MaxConnLifetime = dbMaxConnLifetime
	poolConfig.MaxConnIdleTime = dbMaxConnIdleTime
	poolConfig.HealthCheckPeriod = dbHealthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = dbConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create database connection pool: %w", err)
	}

	// Verify the pool can actually reach the database.
	// pgxpool.NewWithConfig may succeed even if the database is unreachable
	// because connections are created lazily — the ping forces eager check.
	pingCtx, cancel := context.WithTimeout(ctx, dbPingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}