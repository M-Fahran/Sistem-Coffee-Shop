package main

import (
	// "fmt"
	"log"
	// "os"

	"coffeeshop/internal/config"
	"coffeeshop/internal/handler"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	db := config.SetupDatabase()

	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	r := gin.Default()

	handler.SetupRouter(r, userHandler)

	log.Println("🚀 Server Coffee Shop berjalan dengan Gin...")
	r.Run(":8080")
}
