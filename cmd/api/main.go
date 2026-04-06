package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	// 1. Load file .env
	err := godotenv.Load("../../.env") // Sesuaikan path jika file .env ada di root folder
	if err != nil {
		log.Println("Warning: Error loading .env file, using environment variables")
	}

	// 2. Setup Koneksi PostgreSQL
	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)
	
	dbPool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer dbPool.Close()

	// Test Ping ke Database
	if err := dbPool.Ping(context.Background()); err != nil {
		log.Fatalf("Database ping failed: %v\n", err)
	}
	fmt.Println("✅ Connected to PostgreSQL successfully!")

	// 3. Setup Koneksi Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST"),
		Password: os.Getenv("REDIS_PASSWORD"), 
		DB:       0,  
	})

	// Test Ping ke Redis
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Redis ping failed: %v\n", err)
	}
	fmt.Println("✅ Connected to Redis successfully!")

	// 4. Setup Gin Router
	r := gin.Default()

	// Endpoint sederhana untuk test
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "OK",
			"message": "Coffee Shop API is running smoothly!",
		})
	})

	// 5. Jalankan Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("🚀 Server is running on port %s\n", port)
	r.Run(":" + port)
}