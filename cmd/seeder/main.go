package main

import (
	"fmt"
	"log"
	"context"

	"coffeeshop/internal/config"

)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	pool, err := config.NewPostgresPool(ctx, cfg)
	if err != nil {
		log.Fatalf("Gagal konek ke database: %v", err)
	}
	defer pool.Close()

	fmt.Println("Menjalankan seeder")

	query := `
			TRUNCATE TABLE categories, products, users, tables, product_addons, product_addon_map, payment_transactions, orders, order_items, order_item_addons RESTART IDENTITY CASCADE;

			-- Insert Categories
			INSERT INTO categories (id, name) VALUES
			(1, 'Minuman'),
			(2, 'Makanan'),
			(3, 'Snack')
			ON CONFLICT (id) DO NOTHING;
			
			-- Insert Products
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

			-- Insert Users
			INSERT INTO users (email, username, password, role, is_active) VALUES
    		('budi@gmail.com', 'budi', '$2a$10$LpEcc5n3iXQsJMHAhlRE4O3M2L/uyiFiJ/wsQs78m57SmPCI8HGea', 'cashier', true),
    		('bimbim@gmail.com', 'bimbim', '$2a$12$9tTUtc56sA0m9L/cYbITdetHFeDOKBHMvRDKsiz0hzVnvbAwIKEnu', 'admin', true);

			-- Insert Product Addons
			INSERT INTO product_addons (name, price, stock, is_active) VALUES
			('Extra Shot', 5000, 10, true),
			('Syrup Coklat', 3000, 10, true),
			('Syrup Vanila', 3000, 10, true),
			('Whipped Cream', 2000, 15, true),
			('Caramel Drizzle', 2500, 20, true);

			-- Insert Product Addon Map
			INSERT INTO product_addon_map (product_id, product_addon_id) VALUES
			(1, 1), (1, 2), (1, 3), (1, 4),
			(2, 1), (2, 2), (2, 3),
			(3, 1), (3, 2), (3, 4), (3, 5),
			(4, 1), (4, 3), (4, 4);

			-- Insert Tables
			INSERT INTO "tables" (number, qr_token, is_active) VALUES
			('1', 'qr_token_001', true),
			('2', 'qr_token_002', true),
			('3', 'qr_token_003', true),
			('4', 'qr_token_004', true),
			('5', 'qr_token_005', true),
			('6', 'qr_token_006', true),
			('7', 'qr_token_007', true),
			('8', 'qr_token_008', true),
			('Bar 1', 'qr_token_bar_001', true),
			('Bar 2', 'qr_token_bar_002', true);

			-- Insert Payment Transactions
			INSERT INTO payment_transactions (payment_ref, external_id, amount, payment_method, provider, status, handled_by_user_id, user_agent, ip_address, paid_at) VALUES
			('PAY-001', 'EXT-001', 50000.00, 'cash', 'Internal', 'paid', 1, 'Mozilla/5.0', '192.168.1.1', NOW()),
			('PAY-002', 'EXT-002', 75000.00, 'qris', 'QRIS Provider', 'paid', 2, 'Mozilla/5.0', '192.168.1.2', NOW()),
			('PAY-003', 'EXT-003', 100000.00, 'va', 'Bank Transfer', 'paid', 1, 'Mozilla/5.0', '192.168.1.3', NOW()),
			('PAY-004', 'EXT-004', 60000.00, 'ewallet', 'OVO', 'pending', 2, 'Mozilla/5.0', '192.168.1.4', NULL),
			('PAY-005', 'EXT-005', 85000.00, 'cash', 'Internal', 'paid', 1, 'Mozilla/5.0', '192.168.1.5', NOW());

			-- Insert Orders
			INSERT INTO orders (order_number, table_id, customer_name, payment_id, source, created_by_user_id, status, subtotal) VALUES
			('ORD-2024-001', 1, 'John Doe', 1, 'qr', 1, 'completed', 50000.00),
			('ORD-2024-002', 2, 'Jane Smith', 2, 'cashier', 2, 'completed', 75000.00),
			('ORD-2024-003', 3, 'Bob Wilson', 3, 'qr', 1, 'ready', 100000.00),
			('ORD-2024-004', 4, 'Alice Brown', 4, 'cashier', 2, 'preparing', 60000.00),
			('ORD-2024-005', 5, 'Charlie Davis', 5, 'qr', 1, 'completed', 85000.00);

			-- Insert Order Items
			INSERT INTO order_items (order_id, product_id, product_name, unit_price, quantity, subtotal) VALUES
			(1, 3, 'Cappucino', 25000.00, 2, 50000.00),
			(2, 1, 'Espresso', 15000.00, 1, 15000.00),
			(2, 5, 'Nasi Goreng', 20000.00, 3, 60000.00),
			(3, 4, 'Caffe Late', 25000.00, 2, 50000.00),
			(3, 8, 'Kentang Goreng', 35000.00, 1, 35000.00),
			(3, 9, 'Roti Bakar', 15000.00, 1, 15000.00),
			(4, 2, 'Americano', 20000.00, 1, 20000.00),
			(4, 6, 'Nasi Uduk Ayam', 30000.00, 1, 30000.00),
			(4, 7, 'Croissant Almond', 15000.00, 1, 15000.00),
			(5, 1, 'Espresso', 15000.00, 3, 45000.00),
			(5, 3, 'Cappucino', 25000.00, 1, 25000.00),
			(5, 9, 'Roti Bakar', 15000.00, 1, 15000.00);

			-- Insert Order Item Addons
			INSERT INTO order_item_addons (order_item_id, product_addon_id, addon_name, addon_price, quantity, subtotal) VALUES
			(1, 1, 'Extra Shot', 5000.00, 2, 10000.00),
			(1, 4, 'Whipped Cream', 2000.00, 2, 4000.00),
			(2, 2, 'Syrup Coklat', 3000.00, 1, 3000.00),
			(3, 3, 'Syrup Vanila', 3000.00, 3, 9000.00),
			(4, 1, 'Extra Shot', 5000.00, 2, 10000.00),
			(4, 4, 'Whipped Cream', 2000.00, 2, 4000.00),
			(5, 5, 'Caramel Drizzle', 2500.00, 1, 2500.00),
			(6, 3, 'Syrup Vanila', 3000.00, 1, 3000.00),
			(8, 2, 'Syrup Coklat', 3000.00, 1, 3000.00),
			(10, 1, 'Extra Shot', 5000.00, 3, 15000.00),
			(10, 4, 'Whipped Cream', 2000.00, 3, 6000.00),
			(11, 2, 'Syrup Coklat', 3000.00, 1, 3000.00);
	`

	_, err = pool.Exec(ctx, query)
	if err != nil {
		log.Fatal("gagal menjalankan seeder", err)
	}

	fmt.Println("seeder berhasil ditambahkan")
}