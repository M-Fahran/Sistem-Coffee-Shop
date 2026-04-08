package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"github.com/redis/go-redis/v9"
)

func SetupRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_HOST"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB: 0,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Gagal menghubungkan redis: %v\n", err)
	}

	fmt.Println("Redis terhubung")
	return rdb
}