package router

import (
	"coffeeshop/internal/config"
	"coffeeshop/internal/handler"
	"coffeeshop/internal/service"
	"coffeeshop/internal/repository"
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const healthCheckTimeout = 2 * time.Second

// SetupRouter registers all routes on the given Gin engine.
func SetupRouter(r *gin.Engine, pool *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) {
	r.GET("/health", healthHandler(pool, rdb, cfg))
	userRepo := repository.NewUserRepository(pool)
    userService := service.NewUserService(userRepo, cfg.JWT.Secret)
	userHandler := handler.NewUserHandler(userService)

	v1 := r.Group("/api/v1")
	{
		v1.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "pong"})
		})
		v1.POST("/login", userHandler.Login)
	}
}

func healthHandler(pool *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthCheckTimeout)
		defer cancel()

		status := gin.H{
			"app":      "ok",
			"database": "ok",
			"redis":    "ok",
		}
		httpStatus := http.StatusOK

		if err := pool.Ping(ctx); err != nil {
			status["database"] = "error: " + err.Error()
			httpStatus = http.StatusServiceUnavailable
		}

		if err := rdb.Ping(ctx).Err(); err != nil {
			status["redis"] = "error: " + err.Error()
			httpStatus = http.StatusServiceUnavailable
		}

		c.JSON(httpStatus, status)
	}
}