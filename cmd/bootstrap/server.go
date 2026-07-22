package bootstrap

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"coffeeshop/internal/config"
	"coffeeshop/internal/router"
	"coffeeshop/internal/support/exception"
)

// HTTP server timeout constants.
// These prevent slowloris attacks, hung connections, and resource leaks.
const (
	httpReadHeaderTimeout = 5 * time.Second
	httpReadTimeout       = 10 * time.Second
	httpWriteTimeout      = 15 * time.Second
	httpIdleTimeout       = 60 * time.Second
)

// buildHTTPServer constructs the HTTP server with all middleware and routes.
// The returned *http.Server is ready to ListenAndServe.
func buildHTTPServer(cfg *config.Config, deps *dependencies) *http.Server {
	// Set Gin mode based on environment
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Validator setup — pakai json tag name di error messages
	exception.RegisterJSONTagName()

	// Build Gin engine with default middleware (Logger + Recovery)
	route := gin.Default()

	// Add custom middleware
	route.Use(exception.ErrorHandler())

	// Wire routes
	router.SetupRouter(route, deps.pool, deps.rdb, cfg)

	// HTTP server with security timeouts
	return &http.Server{
		Addr:              ":" + cfg.App.Port,
		Handler:           route,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}
}