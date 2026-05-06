package router

import (
	"log/slog"   // ⬅ TAMBAH IMPORT INI
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


	tableRepo := repository.NewTableRepository(pool)
	tableService := service.NewTableService(tableRepo, slog.Default())
	tableController := controller.NewTableController(tableService)

	admin := r.Group("/admin")
	{
		admin.POST("/login", userController.Login)
		adminRoutes := admin.Group("/")
		adminRoutes.Use(middleware.RequireAuth(cfg), middleware.RequireAdmin())
		{
			adminRoutes.POST("/products", productController.CreateProduct)
			adminRoutes.GET("/products", productController.GetAllActive)
			adminRoutes.PUT("/products/:id", productController.UpdateProduct)
		}
	}

	user := r.Group("/user")
	{
		user.GET("/products", productController.GetAllActive)
	}
	table := r.Group("/tables")
	{
		table.POST("", tableController.Create)
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