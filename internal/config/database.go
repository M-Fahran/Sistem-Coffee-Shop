package config

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	dbMaxConnLifetime   = 1 * time.Hour
	dbMaxConnIdleTime   = 30 * time.Minute
	dbHealthCheckPeriod = 1 * time.Minute
	dbConnectTimeout    = 5 * time.Second
	dbPingTimeout       = 5 * time.Second
)

func NewPostgresPool(ctx context.Context, cfg *Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database DSN: %w", err)
	}

	poolConfig.MaxConns = cfg.Database.MaxConns
	poolConfig.MinConns = cfg.Database.MinConns

	poolConfig.MaxConnLifetime = dbMaxConnLifetime
	poolConfig.MaxConnIdleTime = dbMaxConnIdleTime
	poolConfig.HealthCheckPeriod = dbHealthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = dbConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create database connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, dbPingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}