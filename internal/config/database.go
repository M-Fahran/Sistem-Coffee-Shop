package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupDatabase() *pgxpool.Pool {
	// Ambil data dari .env yang sudah dibaca tadi
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	// Rangkai URL koneksinya (Wajib ada sslmode=disable untuk Docker lokal)
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbname)

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatal("Gagal konek ke database:", err)
	}

	return pool
}