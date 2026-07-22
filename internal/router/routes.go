package router

import (
	"log/slog"   
	"coffeeshop/internal/config"
	"coffeeshop/internal/controller"
	"coffeeshop/internal/middleware"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/service"
	"coffeeshop/internal/support/cache"   // ⬅ TAMBAH

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func SetupRouter(r *gin.Engine, pool *pgxpool.Pool, redisClient *redis.Client, cfg *config.Config) {
	userRepo := repository.NewUserRepository(pool)
	userService := service.NewUserService(userRepo, cfg.JWT.Secret)
	userController := controller.NewUserHandler(userService)

	productRepo := repository.NewProductRepository(pool)
	categoriesRepo := repository.NewCategoriesRepository(pool)

	productService := service.NewProductService(productRepo, categoriesRepo, redisClient)
	productController := controller.NewProductController(productService)

	categoriesService := service.NewCategoriesService(categoriesRepo)
	categoriesController := controller.NewCategoriesController(categoriesService)

	productAddOnRepo := repository.NewProductAddOnRepository(pool)
	productAddOnService := service.NewProductAddOnService(productAddOnRepo)
	productAddOnController := controller.NewProductAddOnController(productAddOnService)

	ordersRepo := repository.NewOrdersRepository(pool)
	ordersService := service.NewOrdersService(ordersRepo)
	ordersController := controller.NewOrdersController(ordersService)

	tableRepo := repository.NewTableRepository(pool)
	tableCache := cache.NewTableCache(redisClient)
	tableService := service.NewTableService(tableRepo, tableCache,slog.Default())
	tableController := controller.NewTableController(tableService)

	admin := r.Group("/admin")
	{
		admin.POST("/login", userController.Login)
		adminRoutes := admin.Group("/")
		adminRoutes.Use(middleware.RequireAuth(cfg), middleware.RequireAdmin())
		{
			adminRoutes.GET("/products", productController.GetAllProducts)
			adminRoutes.POST("/products", productController.CreateProduct)
			adminRoutes.PUT("/products/:id", productController.UpdateProduct)
			adminRoutes.DELETE("/products/:id", productController.DeleteProduct)

			adminRoutes.GET("/productsAddOn", productAddOnController.GetAllProductsAddOn)
			adminRoutes.POST("/productsAddOn", productAddOnController.CreateProductAddOn)
			adminRoutes.PUT("/productsAddOn/:id", productAddOnController.UpdateProductAddOn)
			adminRoutes.DELETE("/productsAddOn/:id", productAddOnController.DeleteProductAddOn)

			adminRoutes.GET("/orders", ordersController.GetAllOrders)
			adminRoutes.GET("/orders/:id", ordersController.GetOrderDetail)

			adminRoutes.GET("/categories", categoriesController.GetAllCategories)
			adminRoutes.POST("/categories", categoriesController.CreateCategories)
			adminRoutes.PUT("/categories/:id", categoriesController.UpdateCategories)
			adminRoutes.DELETE("/categories/:id", categoriesController.DeleteCategories)
		}
	}

	user := r.Group("/user")
	{
		user.GET("/products", productController.GetAllProducts)
	}
	table := r.Group("/tables")
	{
		table.GET(
			"", 
			middleware.RateLimitGet(redisClient),
			tableController.List,
		)
		table.GET(
			"/:id", 
			middleware.RateLimitGet(redisClient),
			tableController.GetByID,
		)
		table.POST(
			"", 
			middleware.RateLimitCreate(redisClient),
			tableController.Create,
		)
		table.PATCH(
			"/:id", 
			middleware.RateLimitUpdate(redisClient),
			tableController.Update,
		)
		table.DELETE(
			"/:id", 
			middleware.RateLimitDelete(redisClient),
			tableController.Delete,
		)
		// table.POST("/bulk-delete", tableController.BulkDelete)
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
