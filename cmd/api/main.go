package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"coffeeshop/internal/config"
	"coffeeshop/internal/router"

	"github.com/gin-gonic/gin"
)

// HTTP server Timeout constants
const (
	httpReadHeaderTimeout = 5 * time.Second
	httpReadTimeout       = 10 * time.Second
	httpWriteTimeout      = 15 * time.Second
	httpIdleTimeout       = 60 * time.Second
	shutdownTimeout       = 10 * time.Second
)

func main() {
	// 1. Load config — fail fast if anything is missing or invalid.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ config load failed: %v", err)
	}
	log.Printf("✅ config loaded (env=%s)", cfg.App.Env)

	// Root context for startup — cancelled on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 2. Connect to PostgreSQL.
	pool, err := config.NewPostgresPool(ctx, cfg)
	if err != nil {
		log.Fatalf("❌ postgres connection failed: %v", err)
	}
	defer pool.Close()
	log.Println("✅ postgres connected")

	// 3. Connect to Redis.
	rdb, err := config.NewRedisClient(ctx, cfg)
	if err != nil {
		log.Fatalf("❌ redis connection failed: %v", err)
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Printf("⚠️  redis close error: %v", err)
		}
	}()
	log.Println("✅ redis connected")

	// 4. Setup Gin.
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	route := gin.Default()
	router.SetupRouter(route, pool, rdb, cfg)

	// 5. HTTP server with timeouts — prevents slowloris and hung connections.
	srv := &http.Server{
		Addr:              ":" + cfg.App.Port,
		Handler:           route,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("🚀 server running at http://localhost:%s", cfg.App.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		log.Fatalf("❌ server error: %v", err)
	case <-ctx.Done():
		log.Println("⏳ shutdown signal received, stopping server...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("❌ forced shutdown: %v", err)
		os.Exit(1)
	}

	log.Println("✅ server stopped gracefully")
}
