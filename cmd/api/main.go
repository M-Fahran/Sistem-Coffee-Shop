package main

import (
	"fmt"
	"log"
	"os"

	"coffeeshop/internal/delivery"
	"coffeeshop/internal/config"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("Tidak menemukan file .env")
	}

	db := config.SetupDatabase()
	defer db.Close()

	// redisClient := config.SetupRedis()

	productRepo := repository.NewProductRepository(db)
	productService := service.NewProductService(productRepo)
	productHandler := api.NewProductHandler(productService)

	r := gin.Default()

	api.SetupRouter(r, productHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Server berjalan di port %s\n", port)
	r.Run(":" + port)
}