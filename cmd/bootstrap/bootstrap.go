package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"coffeeshop/internal/config"
)

// Run is the application entry point. It loads config, connects to external
// services, builds the HTTP server, and runs until SIGINT/SIGTERM.
//
// Returns an error if startup fails or if the server crashes (not for
// graceful shutdown).
func Run() error {
	// 1. Load config — fail fast on missing/invalid env vars
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}
	log.Printf("config loaded (env=%s)", cfg.App.Env)

	// 2. Setup signal context — cancelled on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 3. Build dependencies (DB, Redis, etc.)
	deps, err := buildDependencies(ctx, cfg)
	if err != nil {
		return fmt.Errorf("build dependencies: %w", err)
	}
	defer deps.Close()

	// 4. Build HTTP server with all middleware + routes wired
	srv := buildHTTPServer(cfg, deps)

	// 5. Run server with graceful shutdown
	return runServer(ctx, srv, cfg)
}

// dependencies holds external service connections.
// Close() releases all resources in reverse order of acquisition.
type dependencies struct {
	pool *config.Pool
	rdb  *config.RedisClient
}

func (d *dependencies) Close() {
	if d.rdb != nil {
		if err := d.rdb.Close(); err != nil {
			log.Printf("redis close error: %v", err)
		}
	}
	if d.pool != nil {
		d.pool.Close()
	}
}

// buildDependencies connects to all external services.
// On any failure, previously-acquired resources are cleaned up.
func buildDependencies(ctx context.Context, cfg *config.Config) (*dependencies, error) {
	deps := &dependencies{}

	// Postgres
	pool, err := config.NewPostgresPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}
	deps.pool = pool
	log.Println("postgres connected")

	// Redis
	rdb, err := config.NewRedisClient(ctx, cfg)
	if err != nil {
		deps.Close() // cleanup pool yang sudah dibuat
		return nil, fmt.Errorf("redis connect: %w", err)
	}
	deps.rdb = rdb
	log.Println("redis connected")

	return deps, nil
}

// runServer starts the HTTP server in a goroutine and blocks until either:
//   - the server crashes (returns the error)
//   - shutdown signal received (triggers graceful shutdown)
func runServer(ctx context.Context, srv *http.Server, cfg *config.Config) error {
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("server running at http://localhost:%s", cfg.App.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("server crashed: %w", err)
	case <-ctx.Done():
		log.Println("shutdown signal received, stopping server...")
		return shutdown(srv)
	}
}