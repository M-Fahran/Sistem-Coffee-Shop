package bootstrap

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

const shutdownTimeout = 10 * time.Second

// shutdown gracefully stops the HTTP server, waiting up to shutdownTimeout
// for in-flight requests to complete.
//
// Returns an error if shutdown fails (forced kill).
func shutdown(srv *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("forced shutdown: %w", err)
	}

	log.Println("server stopped gracefully")
	return nil
}