package config

import (
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func SetupDatabase(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Gagal konek ke database via GORM:", err)
	}

	return db
}