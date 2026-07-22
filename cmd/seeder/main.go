// Package main — seeder database coffeeshop.
//
// Jalankan dari root project:
//
//	go run ./cmd/seeder
//
// Seeder ini destruktif: semua tabel di-TRUNCATE dulu, lalu diisi ulang.
// Data yang dihasilkan deterministik (fixed random seed), jadi hasilnya
// selalu sama setiap kali dijalankan.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"coffeeshop/internal/config"
)

const (
	totalOrders  = 100
	historyDays  = 30 // transaksi tersebar dalam 30 hari terakhir
	randSeed     = 2026
	seedPassword = "password123" // password semua user hasil seeder
)

// ============================================================================
// Model bantu (bukan entity produksi, khusus seeder)
// ============================================================================

type product struct {
	ID         int64
	CategoryID int64
	Name       string
	Price      float64
	Stock      int
	IsActive   bool
	Addonable  bool // true = minuman, boleh punya add-on
}

type addon struct {
	ID    int64
	Name  string
	Price float64
}

type appUser struct {
	ID       int64
	Username string
	Role     string
	IsActive bool
}

type diningTable struct {
	ID     int64
	Number string
}

type weighted struct {
	value  string
	weight int
}

type seeder struct {
	ctx context.Context
	tx  pgx.Tx
	rnd *rand.Rand

	categories map[string]int64
	products   []product
	addons     []addon
	addonMap   map[int64][]addon // productID -> add-on yang tersedia
	users      []appUser
	cashiers   []appUser
	tables     []diningTable

	sold      map[int64]int // productID -> qty terjual (untuk potong stok)
	orderSeq  map[string]int
	payment   int
	revenue   float64
	itemCount int
	withEmail int
}

// ============================================================================
// Main
// ============================================================================

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load: %v", err)
	}

	ctx := context.Background()

	pool, err := config.NewPostgresPool(ctx, cfg)
	if err != nil {
		log.Fatalf("koneksi database gagal: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	s := &seeder{
		ctx:        ctx,
		tx:         tx,
		rnd:        rand.New(rand.NewSource(randSeed)),
		categories: make(map[string]int64),
		addonMap:   make(map[int64][]addon),
		sold:       make(map[int64]int),
		orderSeq:   make(map[string]int),
	}

	start := time.Now()
	fmt.Println("🧹 Membersihkan tabel lama...")
	s.truncate()

	fmt.Println("📁 Seeding categories...")
	s.seedCategories()

	fmt.Println("☕ Seeding products...")
	s.seedProducts()

	fmt.Println("➕ Seeding product add-ons...")
	s.seedAddons()
	s.seedAddonMap()

	fmt.Println("👤 Seeding users...")
	s.seedUsers()

	fmt.Println("🪑 Seeding tables...")
	s.seedTables()

	fmt.Printf("🧾 Seeding %d transaksi...\n", totalOrders)
	s.seedTransactions()

	fmt.Println("📦 Menyesuaikan stok produk...")
	s.adjustStock()

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit gagal: %v", err)
	}

	s.printSummary(time.Since(start))
}

// ============================================================================
// Truncate
// ============================================================================

func (s *seeder) truncate() {
	s.mustExec(`
		TRUNCATE TABLE
			order_item_addons, order_items, orders, payment_transactions,
			product_addon_map, product_addons, products, categories,
			tables, users
		RESTART IDENTITY CASCADE`)
}

// ============================================================================
// Master data
// ============================================================================

func (s *seeder) seedCategories() {
	for _, name := range []string{"Kopi", "Non-Kopi", "Makanan", "Snack"} {
		id := s.mustID(`INSERT INTO categories (name) VALUES ($1) RETURNING id`, name)
		s.categories[name] = id
	}
}

func (s *seeder) seedProducts() {
	type seedProduct struct {
		category string
		name     string
		price    float64
	}

	drinks := []seedProduct{
		{"Kopi", "Espresso", 15000},
		{"Kopi", "Americano", 20000},
		{"Kopi", "Cappuccino", 25000},
		{"Kopi", "Caffe Latte", 25000},
		{"Kopi", "Flat White", 27000},
		{"Kopi", "Kopi Susu Gula Aren", 22000},
		{"Kopi", "Cold Brew", 28000},
		{"Kopi", "Caffe Mocha", 28000},
		{"Non-Kopi", "Matcha Latte", 27000},
		{"Non-Kopi", "Chocolate Latte", 25000},
		{"Non-Kopi", "Red Velvet Latte", 26000},
		{"Non-Kopi", "Lemon Tea", 15000},
		{"Non-Kopi", "Thai Tea", 20000},
	}

	foods := []seedProduct{
		{"Makanan", "Nasi Goreng Spesial", 32000},
		{"Makanan", "Nasi Uduk Ayam", 30000},
		{"Makanan", "Chicken Katsu Rice", 38000},
		{"Makanan", "Spaghetti Aglio Olio", 35000},
		{"Makanan", "Beef Burger", 45000},
		{"Snack", "Croissant Almond", 22000},
		{"Snack", "Kentang Goreng", 25000},
		{"Snack", "Roti Bakar Coklat", 18000},
		{"Snack", "Cheese Cake Slice", 30000},
		{"Snack", "Banana Bread", 20000},
	}

	insert := func(sp seedProduct, addonable bool, active bool) {
		stock := 80 + s.rnd.Intn(170)
		id := s.mustID(`
			INSERT INTO products (category_id, name, base_price, stock, is_active)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			s.categories[sp.category], sp.name, sp.price, stock, active)

		s.products = append(s.products, product{
			ID:         id,
			CategoryID: s.categories[sp.category],
			Name:       sp.name,
			Price:      sp.price,
			Stock:      stock,
			IsActive:   active,
			Addonable:  addonable,
		})
	}

	for _, d := range drinks {
		insert(d, true, true)
	}
	for _, f := range foods {
		// "Banana Bread" sengaja dibuat non-aktif untuk testing filter is_active
		insert(f, false, f.name != "Banana Bread")
	}
}

func (s *seeder) seedAddons() {
	list := []struct {
		name  string
		price float64
	}{
		{"Extra Shot", 5000},
		{"Syrup Caramel", 5000},
		{"Syrup Vanilla", 5000},
		{"Syrup Hazelnut", 5000},
		{"Whipped Cream", 4000},
		{"Oat Milk", 8000},
		{"Boba", 6000},
		{"Extra Ice", 2000},
	}

	for _, a := range list {
		id := s.mustID(`
			INSERT INTO product_addons (name, price, stock, is_active)
			VALUES ($1, $2, $3, true) RETURNING id`,
			a.name, a.price, 100+s.rnd.Intn(200))

		s.addons = append(s.addons, addon{ID: id, Name: a.name, Price: a.price})
	}
}

// seedAddonMap memberi setiap minuman 3-6 add-on acak.
func (s *seeder) seedAddonMap() {
	for _, p := range s.products {
		if !p.Addonable {
			continue
		}

		pool := make([]addon, len(s.addons))
		copy(pool, s.addons)
		s.rnd.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

		n := 3 + s.rnd.Intn(4)
		selected := pool[:n]

		for _, a := range selected {
			s.mustExec(`
				INSERT INTO product_addon_map (product_id, product_addon_id)
				VALUES ($1, $2)`, p.ID, a.ID)
		}
		s.addonMap[p.ID] = selected
	}
}

func (s *seeder) seedUsers() {
	hashed, err := bcrypt.GenerateFromPassword([]byte(seedPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	list := []struct {
		username string
		role     string
		active   bool
	}{
		{"bimbim", "admin", true},
		{"sari", "admin", true},
		{"budi", "cashier", true},
		{"dimas", "cashier", true},
		{"rina", "cashier", true},
		{"agus", "cashier", true},
		{"dewi", "cashier", true},
		{"fajar", "cashier", true},
		{"nadia", "cashier", true},
		{"rizky", "cashier", false}, // resigned — untuk testing akun non-aktif
	}

	for _, u := range list {
		email := u.username + "@coffeeshop.id"
		id := s.mustID(`
			INSERT INTO users (email, username, password, role, is_active, created_at)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			email, u.username, string(hashed), u.role, u.active,
			time.Now().AddDate(0, 0, -(60 + s.rnd.Intn(120))))

		au := appUser{ID: id, Username: u.username, Role: u.role, IsActive: u.active}
		s.users = append(s.users, au)
		if u.role == "cashier" && u.active {
			s.cashiers = append(s.cashiers, au)
		}
	}
}

func (s *seeder) seedTables() {
	numbers := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "Bar-1", "Bar-2"}

	for i, n := range numbers {
		active := i != len(numbers)-1 // meja terakhir non-aktif (sedang diperbaiki)
		id := s.mustID(`
			INSERT INTO tables (number, qr_token, is_active)
			VALUES ($1, $2, $3) RETURNING id`,
			n, s.randHex(64), active)

		if active {
			s.tables = append(s.tables, diningTable{ID: id, Number: n})
		}
	}
}

// ============================================================================
// Transaksi
// ============================================================================

func (s *seeder) seedTransactions() {
	times := make([]time.Time, 0, totalOrders)
	for i := 0; i < totalOrders; i++ {
		times = append(times, s.randomOrderTime())
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	for _, t := range times {
		s.createOrder(t)
	}
}

func (s *seeder) createOrder(createdAt time.Time) {
	source := s.pick([]weighted{{"qr", 70}, {"cashier", 30}})

	var tableID, createdByUserID *int64

	if source == "qr" {
		t := s.tables[s.rnd.Intn(len(s.tables))]
		tableID = &t.ID
	} else {
		c := s.cashiers[s.rnd.Intn(len(s.cashiers))]
		createdByUserID = &c.ID
		if s.rnd.Float64() < 0.6 { // 60% dine-in, sisanya take away
			t := s.tables[s.rnd.Intn(len(s.tables))]
			tableID = &t.ID
		}
	}

	// --- susun keranjang ---
	type cartAddon struct {
		a        addon
		qty      int
		subtotal float64
	}
	type cartItem struct {
		p        product
		qty      int
		subtotal float64
		addons   []cartAddon
	}

	itemCount := s.rnd.Intn(4) + 1 // 1-4 jenis produk
	picked := s.pickProducts(itemCount)

	var (
		cart     []cartItem
		subtotal float64
	)

	for _, p := range picked {
		qty := s.pickQty()
		item := cartItem{p: p, qty: qty, subtotal: p.Price * float64(qty)}
		subtotal += item.subtotal

		if available := s.addonMap[p.ID]; len(available) > 0 && s.rnd.Float64() < 0.45 {
			nAddon := 1 + s.rnd.Intn(2)
			shuffled := make([]addon, len(available))
			copy(shuffled, available)
			s.rnd.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

			for _, a := range shuffled[:nAddon] {
				ca := cartAddon{a: a, qty: qty, subtotal: a.Price * float64(qty)}
				item.addons = append(item.addons, ca)
				subtotal += ca.subtotal
			}
		}

		cart = append(cart, item)
	}

	// --- identitas pelanggan ---
	customerName := s.customerName()
	customerEmail := s.customerEmail(customerName, source)
	if customerEmail != nil {
		s.withEmail++
	}

	// --- pembayaran ---
	method := s.paymentMethod(source)
	payStatus := s.pick([]weighted{{"paid", 85}, {"pending", 8}, {"expired", 4}, {"failed", 3}})

	var paidAt *time.Time
	if payStatus == "paid" {
		t := createdAt.Add(time.Duration(30+s.rnd.Intn(270)) * time.Second)
		paidAt = &t
	}

	var handledBy *int64
	if source == "cashier" {
		handledBy = createdByUserID
	} else if payStatus == "paid" && s.rnd.Float64() < 0.5 {
		c := s.cashiers[s.rnd.Intn(len(s.cashiers))]
		handledBy = &c.ID
	}

	s.payment++
	paymentRef := fmt.Sprintf("PAY-%s-%04d", createdAt.Format("20060102"), s.payment)
	externalID := fmt.Sprintf("%s-%s", s.providerPrefix(method), s.randHex(12))

	var payload any
	if method != "cash" {
		payload = fmt.Sprintf(
			`{"transaction_id":"%s","gross_amount":"%.2f","payment_type":"%s","transaction_status":"%s"}`,
			externalID, subtotal, method, payStatus)
	}

	paymentID := s.mustID(`
		INSERT INTO payment_transactions
			(payment_ref, external_id, amount, payment_method, provider, status,
			 customer_email, handled_by_user_id, user_agent, ip_address,
			 payload, paid_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id`,
		paymentRef, externalID, subtotal, method, s.provider(method), payStatus,
		customerEmail, handledBy, s.userAgent(source), s.ipAddress(source),
		payload, paidAt, createdAt)

	// --- order ---
	orderStatus := s.orderStatus(payStatus)
	day := createdAt.Format("20060102")
	s.orderSeq[day]++
	orderNumber := fmt.Sprintf("ORD-%s-%03d", day, s.orderSeq[day])

	orderID := s.mustID(`
		INSERT INTO orders
			(order_number, table_id, customer_name, payment_id, source,
			 created_by_user_id, status, subtotal, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		orderNumber, tableID, customerName, paymentID, source,
		createdByUserID, orderStatus, subtotal, createdAt)

	// --- item & add-on ---
	for _, item := range cart {
		itemID := s.mustID(`
			INSERT INTO order_items
				(order_id, product_id, product_name, unit_price, quantity, subtotal)
			VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
			orderID, item.p.ID, item.p.Name, item.p.Price, item.qty, item.subtotal)

		for _, ca := range item.addons {
			s.mustExec(`
				INSERT INTO order_item_addons
					(order_item_id, product_addon_id, addon_name, addon_price, quantity, subtotal)
				VALUES ($1,$2,$3,$4,$5,$6)`,
				itemID, ca.a.ID, ca.a.Name, ca.a.Price, ca.qty, ca.subtotal)
		}

		if orderStatus != "cancelled" {
			s.sold[item.p.ID] += item.qty
		}
		s.itemCount++
	}

	if payStatus == "paid" {
		s.revenue += subtotal
	}
}

// adjustStock memotong stok produk sesuai qty yang terjual.
func (s *seeder) adjustStock() {
	for i, p := range s.products {
		remaining := p.Stock - s.sold[p.ID]
		if remaining < 0 {
			remaining = 0
		}
		s.mustExec(`UPDATE products SET stock = $1 WHERE id = $2`, remaining, p.ID)
		s.products[i].Stock = remaining
	}
}

// ============================================================================
// Helper generator
// ============================================================================

func (s *seeder) randomOrderTime() time.Time {
	now := time.Now()
	day := now.AddDate(0, 0, -s.rnd.Intn(historyDays))

	// jam operasional 08:00 - 21:59
	t := time.Date(day.Year(), day.Month(), day.Day(),
		8+s.rnd.Intn(14), s.rnd.Intn(60), s.rnd.Intn(60), 0, time.Local)

	if t.After(now) {
		t = t.AddDate(0, 0, -1)
	}
	return t
}

func (s *seeder) pickProducts(n int) []product {
	// minuman dimasukkan 2x supaya lebih sering terpilih
	pool := make([]product, 0, len(s.products)*2)
	for _, p := range s.products {
		if !p.IsActive {
			continue
		}
		pool = append(pool, p)
		if p.Addonable {
			pool = append(pool, p)
		}
	}
	s.rnd.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	seen := make(map[int64]bool, n)
	out := make([]product, 0, n)
	for _, p := range pool {
		if seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		out = append(out, p)
		if len(out) == n {
			break
		}
	}
	return out
}

func (s *seeder) pickQty() int {
	switch s.pick([]weighted{{"1", 60}, {"2", 30}, {"3", 10}}) {
	case "1":
		return 1
	case "2":
		return 2
	default:
		return 3
	}
}

func (s *seeder) paymentMethod(source string) string {
	if source == "qr" {
		return s.pick([]weighted{{"qris", 75}, {"ewallet", 20}, {"va", 5}})
	}
	return s.pick([]weighted{{"cash", 55}, {"qris", 30}, {"ewallet", 10}, {"va", 5}})
}

func (s *seeder) provider(method string) string {
	switch method {
	case "qris":
		return "Midtrans"
	case "va":
		return s.pick([]weighted{{"BCA Virtual Account", 40}, {"BNI Virtual Account", 30}, {"Mandiri Virtual Account", 30}})
	case "ewallet":
		return s.pick([]weighted{{"GoPay", 40}, {"OVO", 35}, {"DANA", 25}})
	default:
		return "Internal"
	}
}

func (s *seeder) providerPrefix(method string) string {
	switch method {
	case "qris":
		return "QRIS"
	case "va":
		return "VA"
	case "ewallet":
		return "EW"
	default:
		return "CASH"
	}
}

// orderStatus menurunkan status order dari status pembayaran agar data konsisten.
func (s *seeder) orderStatus(payStatus string) string {
	switch payStatus {
	case "paid":
		return s.pick([]weighted{
			{"completed", 70}, {"ready", 12}, {"preparing", 10}, {"confirmed", 8},
		})
	case "pending":
		return "pending"
	default: // failed / expired
		return "cancelled"
	}
}

func (s *seeder) customerName() string {
	first := []string{
		"Andi", "Budi", "Citra", "Dewi", "Eka", "Farhan", "Gita", "Hendra",
		"Indah", "Joko", "Kiki", "Laras", "Maya", "Nanda", "Oscar", "Putri",
		"Rangga", "Sinta", "Tono", "Vina", "Wawan", "Yuni", "Zaki", "Rizal",
	}
	last := []string{
		"Pratama", "Wijaya", "Santoso", "Nugroho", "Hidayat", "Saputra",
		"Kusuma", "Permata", "Anggraini", "Halim", "Maulana", "Setiawan",
	}
	return first[s.rnd.Intn(len(first))] + " " + last[s.rnd.Intn(len(last))]
}

// customerEmail menurunkan email dari nama pelanggan.
// Nullable: pesanan via QR hampir selalu isi email (buat struk digital),
// pesanan kasir sering tidak diisi karena bayar cash langsung.
func (s *seeder) customerEmail(name, source string) *string {
	fillRate := 0.4
	if source == "qr" {
		fillRate = 0.9
	}
	if s.rnd.Float64() > fillRate {
		return nil
	}

	local := strings.ToLower(strings.ReplaceAll(name, " ", "."))
	domains := []string{"gmail.com", "yahoo.com", "outlook.com", "proton.me"}

	email := fmt.Sprintf("%s%d@%s",
		local, 10+s.rnd.Intn(90), domains[s.rnd.Intn(len(domains))])
	return &email
}

func (s *seeder) userAgent(source string) string {
	if source == "cashier" {
		return "CoffeeShop-POS/1.0 (Windows NT 10.0; Win64; x64)"
	}
	agents := []string{
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36",
	}
	return agents[s.rnd.Intn(len(agents))]
}

func (s *seeder) ipAddress(source string) string {
	if source == "cashier" {
		return fmt.Sprintf("192.168.1.%d", 2+s.rnd.Intn(50))
	}
	return fmt.Sprintf("%d.%d.%d.%d",
		100+s.rnd.Intn(100), s.rnd.Intn(256), s.rnd.Intn(256), 1+s.rnd.Intn(254))
}

func (s *seeder) randHex(n int) string {
	const hexChars = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hexChars[s.rnd.Intn(len(hexChars))]
	}
	return string(b)
}

func (s *seeder) pick(items []weighted) string {
	total := 0
	for _, it := range items {
		total += it.weight
	}
	n := s.rnd.Intn(total)
	for _, it := range items {
		n -= it.weight
		if n < 0 {
			return it.value
		}
	}
	return items[len(items)-1].value
}

// ============================================================================
// DB helper
// ============================================================================

func (s *seeder) mustExec(query string, args ...any) {
	if _, err := s.tx.Exec(s.ctx, query, args...); err != nil {
		log.Fatalf("exec gagal: %v\nquery: %s", err, query)
	}
}

func (s *seeder) mustID(query string, args ...any) int64 {
	var id int64
	if err := s.tx.QueryRow(s.ctx, query, args...).Scan(&id); err != nil {
		log.Fatalf("query gagal: %v\nquery: %s", err, query)
	}
	return id
}

// ============================================================================
// Summary
// ============================================================================

func (s *seeder) printSummary(elapsed time.Duration) {
	fmt.Println()
	fmt.Println("🚀 SEEDER SUCCESS")
	fmt.Println("────────────────────────────────────")
	fmt.Printf("  Categories        : %d\n", len(s.categories))
	fmt.Printf("  Products          : %d\n", len(s.products))
	fmt.Printf("  Product add-ons   : %d\n", len(s.addons))
	fmt.Printf("  Users             : %d (%d kasir aktif)\n", len(s.users), len(s.cashiers))
	fmt.Printf("  Tables (aktif)    : %d\n", len(s.tables))
	fmt.Printf("  Orders            : %d\n", totalOrders)
	fmt.Printf("  Order items       : %d\n", s.itemCount)
	fmt.Printf("  Punya email       : %d dari %d transaksi\n", s.withEmail, totalOrders)
	fmt.Printf("  Revenue (paid)    : Rp %.0f\n", s.revenue)
	fmt.Printf("  Durasi            : %s\n", elapsed.Round(time.Millisecond))
	fmt.Println("────────────────────────────────────")
	fmt.Printf("  Login: bimbim@coffeeshop.id / %s (admin)\n", seedPassword)
	fmt.Printf("         budi@coffeeshop.id   / %s (cashier)\n", seedPassword)
}