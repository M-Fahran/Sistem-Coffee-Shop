package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"coffeeshop/internal/config"
	"coffeeshop/internal/controller"
	"coffeeshop/internal/middleware"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/service"
	"coffeeshop/internal/support/cache"
	"coffeeshop/internal/support/token"
)

const (
	roleAdmin   = "admin"
	roleCashier = "cashier"
)

func SetupRouter(r *gin.Engine, pool *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) {
	tm := token.NewManager(cfg)
	rev := cache.NewRevocationStore(rdb)
	c := buildControllers(pool, rdb, tm, rev)

	api := r.Group("/api/v1")

	registerPublicRoutes(api, rdb, c)
	registerCustomerRoutes(api, rdb, tm, rev, c)
	registerStaffRoutes(api, rdb, tm, rev, c)
}

// ============================================================================
// Wiring
// ============================================================================

type controllers struct {
	auth         *controller.UserHandler
	qr           *controller.QRController
	table        *controller.TableController
	product      *controller.ProductController
	productAddOn *controller.ProductAddOnController
	categories   *controller.CategoriesController
	orders       *controller.OrdersController
}

func buildControllers(
	pool *pgxpool.Pool,
	rdb *redis.Client,
	tm *token.Manager,
	rev *cache.RevocationStore,
) *controllers {
	log := slog.Default()

	userRepo := repository.NewUserRepository(pool)
	productRepo := repository.NewProductRepository(pool)
	productAddOnRepo := repository.NewProductAddOnRepository(pool)
	categoriesRepo := repository.NewCategoriesRepository(pool)
	ordersRepo := repository.NewOrdersRepository(pool)
	tableRepo := repository.NewTableRepository(pool)
	idemStore := cache.NewIdempotencyStore(rdb)   // ⬅ tambah ini

	tableSvc := service.NewTableService(tableRepo, cache.NewTableCache(rdb), rev, tm, log)
	userSvc := service.NewUserService(userRepo, tm, rev)
	

	return &controllers{
		auth:         controller.NewUserHandler(userSvc),
		qr:           controller.NewQRController(tableSvc),
		table:        controller.NewTableController(tableSvc),
		product:      controller.NewProductController(service.NewProductService(productRepo, categoriesRepo, rdb)),
		productAddOn: controller.NewProductAddOnController(service.NewProductAddOnService(productAddOnRepo)),
		categories:   controller.NewCategoriesController(service.NewCategoriesService(categoriesRepo)),
		orders: controller.NewOrdersController(
			service.NewOrdersService(ordersRepo, idemStore, log, "FakeGateway"),
		),
	}
}

// ============================================================================
// Publik — tanpa auth
// ============================================================================

func registerPublicRoutes(api *gin.RouterGroup, rdb *redis.Client, c *controllers) {
	api.POST("/auth/login", middleware.RateLimitLogin(rdb), c.auth.Login)

	// Penukaran QR token → session pelanggan.
	api.POST("/qr/:token/session", middleware.RateLimitPublic(rdb), c.qr.CreateSession)
}

// ============================================================================
// Pelanggan — butuh session token hasil scan QR
// ============================================================================

func registerCustomerRoutes(
	api *gin.RouterGroup,
	rdb *redis.Client,
	tm *token.Manager,
	rev *cache.RevocationStore,
	c *controllers,
) {
	customer := api.Group("/customer")
	customer.Use(middleware.RequireTableSession(tm, rev))
	{
		customer.GET("/menu", middleware.RateLimitGet(rdb), c.product.GetAllProducts)

		// Nanti diisi:
		//   customer.POST("/orders", ...)   → table_id dari session, bukan body
		//   customer.GET("/orders/:id", ...)
	}
}

// ============================================================================
// Staff — butuh access token; sebagian khusus admin
// ============================================================================

func registerStaffRoutes(
	api *gin.RouterGroup,
	rdb *redis.Client,
	tm *token.Manager,
	rev *cache.RevocationStore,
	c *controllers,
) {
	staff := api.Group("")
	staff.Use(middleware.RequireStaff(tm, rev))

	// --- kasir & admin ---
	{
		staff.POST("/auth/logout", middleware.RateLimitLogout(rdb), c.auth.Logout)

		staff.GET("/orders", middleware.RateLimitGet(rdb), c.orders.GetAllOrders)
		staff.GET("/orders/:id", middleware.RateLimitGet(rdb), c.orders.GetOrderDetail)
		staff.GET("/products", middleware.RateLimitGet(rdb), c.product.GetAllProducts)
		staff.GET("/categories", middleware.RateLimitGet(rdb), c.categories.GetAllCategories)

		// Kasir mengosongkan meja — memutus session pelanggan yang sudah pergi.
		// Sengaja bukan admin-only: ini operasi harian kasir.
		staff.POST("/tables/:id/end-sessions",
			middleware.RateLimitUpdate(rdb), c.table.EndSessions)
	}

	// --- khusus admin ---
	admin := staff.Group("")
	admin.Use(middleware.RequireRole(roleAdmin))
	{
		registerTableRoutes(admin, rdb, c)
		registerProductRoutes(admin, rdb, c)
		registerCategoryRoutes(admin, rdb, c)
	}
}

func registerTableRoutes(admin *gin.RouterGroup, rdb *redis.Client, c *controllers) {
	tables := admin.Group("/tables")
	{
		tables.GET("", middleware.RateLimitGet(rdb), c.table.List)
		tables.GET("/:id", middleware.RateLimitGet(rdb), c.table.GetByID)
		tables.POST("", middleware.RateLimitCreate(rdb), c.table.Create)
		tables.PATCH("/:id", middleware.RateLimitUpdate(rdb), c.table.Update)
		tables.DELETE("/:id", middleware.RateLimitDelete(rdb), c.table.Delete)

		// Pemulihan kalau stiker QR bocor: token baru + semua session dicabut.
		tables.POST("/:id/rotate-qr", middleware.RateLimitUpdate(rdb), c.table.RotateQR)
	}
}

func registerProductRoutes(admin *gin.RouterGroup, rdb *redis.Client, c *controllers) {
	products := admin.Group("/products")
	{
		products.POST("", middleware.RateLimitCreate(rdb), c.product.CreateProduct)
		products.PUT("/:id", middleware.RateLimitUpdate(rdb), c.product.UpdateProduct)
		products.DELETE("/:id", middleware.RateLimitDelete(rdb), c.product.DeleteProduct)
	}

	addons := admin.Group("/product-addons")
	{
		addons.GET("", middleware.RateLimitGet(rdb), c.productAddOn.GetAllProductsAddOn)
		addons.POST("", middleware.RateLimitCreate(rdb), c.productAddOn.CreateProductAddOn)
		addons.PUT("/:id", middleware.RateLimitUpdate(rdb), c.productAddOn.UpdateProductAddOn)
		addons.DELETE("/:id", middleware.RateLimitDelete(rdb), c.productAddOn.DeleteProductAddOn)
	}
}

func registerCategoryRoutes(admin *gin.RouterGroup, rdb *redis.Client, c *controllers) {
	categories := admin.Group("/categories")
	{
		categories.POST("", middleware.RateLimitCreate(rdb), c.categories.CreateCategories)
		categories.PUT("/:id", middleware.RateLimitUpdate(rdb), c.categories.UpdateCategories)
		categories.DELETE("/:id", middleware.RateLimitDelete(rdb), c.categories.DeleteCategories)
	}
}