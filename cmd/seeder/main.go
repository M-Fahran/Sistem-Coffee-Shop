package main

import (
	"context"
	"fmt"
	"log"

	"coffeeshop/internal/config"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("tidak menemukan file env")
	}

	db := config.SetupDatabase()
	defer db.Close()

	fmt.Println("Menjalankan seeder")

	query := `
			TRUNCATE TABLE categories, products, users RESTART IDENTITY CASCADE;

			INSERT INTO categories (id, name) VALUES
			(1, 'Minuman'),
			(2, 'Makanan'),
			(3, 'Snack')
			ON CONFLICT (id) DO NOTHING;
			
			INSERT INTO products (category_id, name, base_price, stock, is_active) VALUES
			(1, 'Espresso', 15000, 100, true),
			(1, 'Americano', 20000, 50, true),
			(1, 'Cappucino', 25000, 90, true),
			(1, 'Caffe Late', 25000, 50, true),
			(2, 'Nasi Goreng', 20000, 90, true),
			(2, 'Nasi Uduk Ayam', 30000, 100, true),
			(3, 'Croissant Almond', 15000, 50, true),
			(3, 'Kentang Goreng', 35000, 100, true),
			(3, 'Roti Bakar', 15000, 100, true);

			INSERT INTO users (email, username, password, role, is_active) VALUES
    		('budi@gmail.com', 'budi', '$2a$12$RWh.ocubHv6jnSCRZvId6epsh/xiEPKkmwh4bgajSxXEzn88gV/dS', 'cashier', true);
	`

	_, err := db.Exec(context.Background(), query)
	if err != nil {
		log.Fatal("gagal menjalankan seeder", err)
	}

	fmt.Println("seeder berhasil ditambahkan")
}