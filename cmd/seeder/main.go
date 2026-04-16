package main

import (
	"fmt"
	"log"

	"coffeeshop/internal/config"
	"coffeeshop/internal/entity"

)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db := config.SetupDatabase(cfg.DatabaseDSN())
	sqlDB, err := db.DB()
	if err == nil {
    	defer sqlDB.Close()
}

	fmt.Println("Menjalankan seeder")

	db.AutoMigrate(&entity.User{}, &entity.Category{}, &entity.Product{})

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
    		('budi@gmail.com', 'budi', '$2a$10$LpEcc5n3iXQsJMHAhlRE4O3M2L/uyiFiJ/wsQs78m57SmPCI8HGea', 'cashier', true);
	`

	err = db.Exec(query).Error
	if err != nil {
		log.Fatal("gagal menjalankan seeder", err)
	}

	fmt.Println("seeder berhasil ditambahkan")
}