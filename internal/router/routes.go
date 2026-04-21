package router

import (
	"coffeeshop/internal/config"
	"coffeeshop/internal/controller"
	"coffeeshop/internal/middleware"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/service"
	// "context"
	// "net/http"
	// "time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// const healthCheckTimeout = 2 * time.Second

// SetupRouter registers all routes on the given Gin engine.
func SetupRouter(r *gin.Engine, pool *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) {
	userRepo := repository.NewUserRepository(pool)
    userService := service.NewUserService(userRepo, cfg.JWT.Secret)
	userController := controller.NewUserHandler(userService)

	productRepo := repository.NewProductRepository(pool)
	productService := service.NewProductService(productRepo)
	productController := controller.NewProductController(productService)

	api := r.Group("/api/v1")
	{
		api.POST("/login", userController.Login)
		adminRoutes := api.Group("/")
		adminRoutes.Use(middleware.RequireAuth(cfg), middleware.RequireAdmin())
		{
			// POST /api/v1/products
			adminRoutes.POST("/products", productController.CreateProduct)
		}
	}
}

// func healthHandler(pool *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		ctx, cancel := context.WithTimeout(c.Request.Context(), healthCheckTimeout)
// 		defer cancel()

// 		status := gin.H{
// 			"app":      "ok",
// 			"database": "ok",
// 			"redis":    "ok",
// 		}
// 		httpStatus := http.StatusOK

// 		if err := pool.Ping(ctx); err != nil {
// 			status["database"] = "error: " + err.Error()
// 			httpStatus = http.StatusServiceUnavailable
// 		}

// 		if err := rdb.Ping(ctx).Err(); err != nil {
// 			status["redis"] = "error: " + err.Error()
// 			httpStatus = http.StatusServiceUnavailable
// 		}

// 		c.JSON(httpStatus, status)
// 	}
// }