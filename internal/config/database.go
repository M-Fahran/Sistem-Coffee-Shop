package config

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ============================================================================
// Type aliases — repositories use these instead of importing pgx directly.
// All driver-specific types are centralized here for easier swapping later.
// ============================================================================

// Pool is the database connection pool.
type Pool = pgxpool.Pool

// PgError is the PostgreSQL driver error used for SQLSTATE inspection.
type PgError = pgconn.PgError

// ErrNoRows is returned when a query expecting a row finds none.
var ErrNoRows = pgx.ErrNoRows

// ============================================================================
// PostgreSQL SQLSTATE codes commonly inspected at the repository layer.
// ============================================================================

const (
	PgErrUniqueViolation     = "23505"
	PgErrForeignKeyViolation = "23503"
	PgErrCheckViolation      = "23514"
	PgErrNotNullViolation    = "23502"
)

// ============================================================================
// Pool configuration constants.
// ============================================================================

const (
	dbMaxConnLifetime   = 1 * time.Hour
	dbMaxConnIdleTime   = 30 * time.Minute
	dbHealthCheckPeriod = 1 * time.Minute
	dbConnectTimeout    = 5 * time.Second
	dbPingTimeout       = 5 * time.Second
)

// NewPostgresPool creates a configured pgx connection pool and verifies connectivity.
func NewPostgresPool(ctx context.Context, cfg *Config) (*Pool, error) {
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