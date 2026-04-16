package main

import (
	// "fmt"
	"log"
	// "os"

	"coffeeshop/internal/config"
	"coffeeshop/internal/entity"
	"coffeeshop/internal/handler"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Gagal load config: %v", err)
	}

	db := config.SetupDatabase(cfg.DatabaseDSN())
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	db.AutoMigrate(&entity.User{})

	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo, cfg.App.Port)
	userHandler := handler.NewUserHandler(userService)

	r := gin.Default()
	handler.SetupRouter(r, userHandler)

	log.Println("🚀 Server Coffee Shop berjalan dengan Gin...")
	r.Run(":8080")
}
