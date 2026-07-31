package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"coffeeshop/internal/config"
	"coffeeshop/internal/entity"
	"coffeeshop/internal/support/exception"
)

// orderTxTimeout membatasi berapa lama satu transaksi boleh memegang lock
// baris produk. Tanpa ini, satu koneksi yang menggantung bisa memblokir
// semua pesanan untuk produk yang sama.
const orderTxTimeout = 5 * time.Second

// Kolom orders selalu diambil dalam urutan ini supaya scan-nya konsisten
// antar query. Kalau menambah kolom, ubah di sini dan di scanOrder().
const orderColumns = `
	id, order_number, table_id, customer_name, payment_id, source,
	created_by_user_id, status, subtotal, created_at, updated_at`

type OrdersRepository struct {
	db *config.Pool
}

func NewOrdersRepository(db *config.Pool) *OrdersRepository {
	return &OrdersRepository{db: db}
}

// ============================================================================
// READ
// ============================================================================

// GetAllOrders mengambil daftar pesanan terbaru.
//
// CATATAN: belum ada pagination. Aman untuk data seeder, tapi wajib ditambah
// sebelum produksi — kalau tidak, satu request bisa menarik puluhan ribu
// baris sekaligus.
func (r *OrdersRepository) GetAllOrders(ctx context.Context) ([]entity.Order, error) {
	query := `SELECT ` + orderColumns + ` FROM orders ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("orders list: %w", err))
	}
	defer rows.Close()

	orders := make([]entity.Order, 0, 32)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, exception.Internal(fmt.Errorf("orders list rows: %w", err))
	}
	return orders, nil
}

// GetOrderByID mengambil satu pesanan tanpa item-nya.
func (r *OrdersRepository) GetOrderByID(ctx context.Context, orderID int64) (entity.Order, error) {
	query := `SELECT ` + orderColumns + ` FROM orders WHERE id = $1`

	order, err := scanOrder(r.db.QueryRow(ctx, query, orderID))
	if err != nil {
		if errors.Is(err, config.ErrNoRows) {
			return entity.Order{}, exception.NotFound("ORDER_404", "order not found")
		}
		return entity.Order{}, err
	}
	return order, nil
}

// GetItemsByOrderID mengambil item beserta add-on-nya.
//
// Dua query, bukan N+1: satu untuk item, satu untuk seluruh add-on milik
// pesanan tersebut, lalu digabung di memori.
func (r *OrdersRepository) GetItemsByOrderID(ctx context.Context, orderID int64) ([]entity.OrderItem, error) {
	const itemQuery = `
		SELECT id, order_id, product_id, product_name, unit_price, quantity, subtotal, notes
		FROM order_items
		WHERE order_id = $1
		ORDER BY id`

	rows, err := r.db.Query(ctx, itemQuery, orderID)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("order items: %w", err))
	}
	defer rows.Close()

	items := make([]entity.OrderItem, 0, 8)
	index := make(map[int64]int, 8) // orderItemID -> posisi di slice

	for rows.Next() {
		var it entity.OrderItem
		if err := rows.Scan(
			&it.ID, &it.OrderID, &it.ProductID, &it.ProductName,
			&it.UnitPrice, &it.Quantity, &it.Subtotal, &it.Notes,
		); err != nil {
			return nil, exception.Internal(fmt.Errorf("order items scan: %w", err))
		}
		index[it.ID] = len(items)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, exception.Internal(fmt.Errorf("order items rows: %w", err))
	}
	if len(items) == 0 {
		return items, nil
	}

	addons, err := r.getAddonsByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	for _, a := range addons {
		if pos, ok := index[a.OrderItemID]; ok {
			items[pos].Addons = append(items[pos].Addons, a)
		}
	}
	return items, nil
}

func (r *OrdersRepository) getAddonsByOrderID(ctx context.Context, orderID int64) ([]entity.OrderItemAddon, error) {
	const query = `
		SELECT oia.id, oia.order_item_id, oia.product_addon_id,
		       oia.addon_name, oia.addon_price, oia.quantity, oia.subtotal
		FROM order_item_addons oia
		JOIN order_items oi ON oi.id = oia.order_item_id
		WHERE oi.order_id = $1
		ORDER BY oia.id`

	rows, err := r.db.Query(ctx, query, orderID)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("order item addons: %w", err))
	}
	defer rows.Close()

	addons := make([]entity.OrderItemAddon, 0, 8)
	for rows.Next() {
		var a entity.OrderItemAddon
		if err := rows.Scan(
			&a.ID, &a.OrderItemID, &a.ProductAddonID,
			&a.AddonName, &a.AddonPrice, &a.Quantity, &a.Subtotal,
		); err != nil {
			return nil, exception.Internal(fmt.Errorf("order item addons scan: %w", err))
		}
		addons = append(addons, a)
	}
	if err := rows.Err(); err != nil {
		return nil, exception.Internal(fmt.Errorf("order item addons rows: %w", err))
	}
	return addons, nil
}

// ============================================================================
// WRITE — CreateOrder
// ============================================================================

// CreateOrder membuat pesanan secara atomik.
//
// Seluruh langkah berada dalam satu transaksi:
//
//  1. Kunci baris produk (FOR UPDATE, urut ID untuk mencegah deadlock)
//  2. Ambil harga dari database — bukan dari client
//  3. Validasi stok dan kesesuaian add-on dengan produknya
//  4. Insert payment → order → items → addons
//  5. Kurangi stok
//
// Kalau salah satu gagal, tidak ada yang tersisa separuh jadi.
func (r *OrdersRepository) CreateOrder(ctx context.Context, in entity.CreateOrderInput) (*entity.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, orderTxTimeout)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("order tx begin: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()

	products, err := lockOrderProducts(ctx, tx, in.ProductIDs())
	if err != nil {
		return nil, err
	}

	addons, err := loadOrderAddons(ctx, tx, in.ProductIDs())
	if err != nil {
		return nil, err
	}

	items, subtotal, err := priceOrderItems(in.Items, products, addons)
	if err != nil {
		return nil, err
	}

	paymentID, err := insertOrderPayment(ctx, tx, in, subtotal)
	if err != nil {
		return nil, err
	}

	order, err := insertOrderRow(ctx, tx, in, paymentID, subtotal)
	if err != nil {
		return nil, err
	}

	if err := insertOrderItems(ctx, tx, order.ID, items); err != nil {
		return nil, err
	}

	if err := decreaseProductStock(ctx, tx, items); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, exception.Internal(fmt.Errorf("order tx commit: %w", err))
	}

	order.Items = toEntityOrderItems(order.ID, items)
	return order, nil
}

// ============================================================================
// WRITE — UpdateStatus
// ============================================================================

// UpdateStatus memindahkan pesanan ke status berikutnya.
//
// Barisnya dikunci dulu supaya dua kasir yang menekan tombol bersamaan tidak
// menghasilkan transisi ganda. Keabsahan transisi diperiksa dari status yang
// baru saja dibaca di dalam lock — bukan dari status yang dikirim klien.
func (r *OrdersRepository) UpdateStatus(
	ctx context.Context,
	orderID int64,
	next config.OrderStatus,
) (*entity.Order, error) {
	ctx, cancel := context.WithTimeout(ctx, orderTxTimeout)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("status tx begin: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current config.OrderStatus
	err = tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&current)
	if err != nil {
		if errors.Is(err, config.ErrNoRows) {
			return nil, exception.NotFound("ORDER_404", "order not found")
		}
		return nil, exception.Internal(fmt.Errorf("lock order: %w", err))
	}

	if current == next {
		return nil, exception.Conflict("ORDER_409",
			fmt.Sprintf("order is already %s", next.String()))
	}
	if !current.CanTransitionTo(next) {
		return nil, exception.Conflict("ORDER_409",
			fmt.Sprintf("cannot change status from %s to %s", current.String(), next.String()))
	}

	// Pembatalan mengembalikan stok. Tanpa ini, pesanan yang dibatalkan
	// tetap "memakan" stok selamanya.
	if next == config.OrderStatusCancelled {
		if err := restoreProductStock(ctx, tx, orderID); err != nil {
			return nil, err
		}
	}

	const updateQuery = `
		UPDATE orders SET status = $1, updated_at = now()
		WHERE id = $2
		RETURNING ` + orderColumns

	order, err := scanOrder(tx.QueryRow(ctx, updateQuery, next, orderID))
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("update order status: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, exception.Internal(fmt.Errorf("status tx commit: %w", err))
	}
	return &order, nil
}

// ============================================================================
// Tipe internal untuk penulisan
// ============================================================================

type lockedProduct struct {
	ID    int64
	Name  string
	Price int64
	Stock int
}

type availableAddon struct {
	ID    int64
	Name  string
	Price int64
}

type pricedAddon struct {
	addon    availableAddon
	quantity int
	subtotal int64
}

type pricedItem struct {
	product  lockedProduct
	quantity int
	notes    *string
	subtotal int64
	addons   []pricedAddon

	// persistedID diisi setelah baris order_items benar-benar tertulis.
	persistedID int64
}

// ============================================================================
// Langkah-langkah transaksi
// ============================================================================

// lockOrderProducts mengunci baris produk yang dipesan.
//
// ORDER BY id penting: kalau dua transaksi mengunci produk yang sama dengan
// urutan berbeda, keduanya bisa saling menunggu (deadlock). Urutan yang
// konsisten menghilangkan kemungkinan itu.
func lockOrderProducts(ctx context.Context, tx pgx.Tx, ids []int64) (map[int64]lockedProduct, error) {
	const query = `
		SELECT id, name, base_price, stock
		FROM products
		WHERE id = ANY($1) AND is_active = TRUE
		ORDER BY id
		FOR UPDATE`

	rows, err := tx.Query(ctx, query, ids)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("lock products: %w", err))
	}
	defer rows.Close()

	products := make(map[int64]lockedProduct, len(ids))
	for rows.Next() {
		var p lockedProduct
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock); err != nil {
			return nil, exception.Internal(fmt.Errorf("lock products scan: %w", err))
		}
		products[p.ID] = p
	}
	if err := rows.Err(); err != nil {
		return nil, exception.Internal(fmt.Errorf("lock products rows: %w", err))
	}

	// Produk yang tidak ketemu berarti tidak ada atau sudah dinonaktifkan.
	// Keduanya dijawab sama supaya tidak bisa dipakai memetakan katalog.
	for _, id := range ids {
		if _, ok := products[id]; !ok {
			return nil, exception.BadRequest("ORDER_400",
				fmt.Sprintf("product %d is not available", id))
		}
	}
	return products, nil
}

// loadOrderAddons mengambil add-on yang SAH untuk tiap produk.
//
// Hasilnya dipakai memverifikasi bahwa add-on yang diminta memang terdaftar
// untuk produk itu — tanpa ini, pelanggan bisa menambahkan "Extra Shot"
// ke Nasi Goreng, atau menempelkan add-on ke produk yang tidak menyediakannya.
func loadOrderAddons(ctx context.Context, tx pgx.Tx, productIDs []int64) (map[int64]map[int64]availableAddon, error) {
	const query = `
		SELECT pam.product_id, pa.id, pa.name, pa.price
		FROM product_addon_map pam
		JOIN product_addons pa ON pa.id = pam.product_addon_id
		WHERE pam.product_id = ANY($1) AND pa.is_active = TRUE`

	rows, err := tx.Query(ctx, query, productIDs)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("load addons: %w", err))
	}
	defer rows.Close()

	result := make(map[int64]map[int64]availableAddon, len(productIDs))
	for rows.Next() {
		var (
			productID int64
			a         availableAddon
		)
		if err := rows.Scan(&productID, &a.ID, &a.Name, &a.Price); err != nil {
			return nil, exception.Internal(fmt.Errorf("load addons scan: %w", err))
		}
		if result[productID] == nil {
			result[productID] = make(map[int64]availableAddon, 4)
		}
		result[productID][a.ID] = a
	}
	if err := rows.Err(); err != nil {
		return nil, exception.Internal(fmt.Errorf("load addons rows: %w", err))
	}
	return result, nil
}

// priceOrderItems menghitung harga dari data database dan memvalidasi stok.
//
// Stok diakumulasi per produk lebih dulu: kalau produk yang sama muncul di
// dua baris keranjang, yang dicek adalah totalnya, bukan masing-masing.
func priceOrderItems(
	inputs []entity.OrderItemInput,
	products map[int64]lockedProduct,
	addons map[int64]map[int64]availableAddon,
) ([]pricedItem, int64, error) {
	needed := make(map[int64]int, len(products))
	for _, in := range inputs {
		needed[in.ProductID] += in.Quantity
	}
	for productID, qty := range needed {
		p := products[productID]
		if p.Stock < qty {
			return nil, 0, exception.Conflict("ORDER_409",
				fmt.Sprintf("insufficient stock for %s (available: %d)", p.Name, p.Stock))
		}
	}

	items := make([]pricedItem, 0, len(inputs))
	var subtotal int64

	for _, in := range inputs {
		p := products[in.ProductID]

		item := pricedItem{
			product:  p,
			quantity: in.Quantity,
			notes:    in.Notes,
			subtotal: p.Price * int64(in.Quantity),
		}
		subtotal += item.subtotal

		for _, addonID := range in.AddonIDs {
			available, ok := addons[in.ProductID][addonID]
			if !ok {
				return nil, 0, exception.BadRequest("ORDER_400",
					fmt.Sprintf("add-on %d is not available for %s", addonID, p.Name))
			}

			// Jumlah add-on mengikuti jumlah produknya — 2 kopi dengan
			// extra shot berarti 2 extra shot.
			priced := pricedAddon{
				addon:    available,
				quantity: in.Quantity,
				subtotal: available.Price * int64(in.Quantity),
			}
			item.addons = append(item.addons, priced)
			subtotal += priced.subtotal
		}

		items = append(items, item)
	}

	return items, subtotal, nil
}

func insertOrderPayment(ctx context.Context, tx pgx.Tx, in entity.CreateOrderInput, amount int64) (int64, error) {
	// payment_ref dibuat dari sequence supaya unik tanpa perlu menghitung
	// baris atau mengambil lock tambahan.
	const query = `
		INSERT INTO payment_transactions
			(payment_ref, amount, payment_method, provider, status,
			 customer_email, handled_by_user_id, user_agent, ip_address)
		VALUES (
			'PAY-' || to_char(now(), 'YYYYMMDD') || '-' ||
			         lpad(nextval('payment_ref_seq')::text, 6, '0'),
			$1, $2, $3, $4, $5, $6, $7, $8
		)
		RETURNING id`

	var paymentID int64
	err := tx.QueryRow(ctx, query,
		amount,
		in.PaymentMethod,
		in.Provider,
		config.PaymentStatusPending, // paid_at tetap NULL — dijaga CHECK constraint
		in.CustomerEmail,
		in.CreatedByUserID,
		in.UserAgent,
		in.IPAddress,
	).Scan(&paymentID)
	if err != nil {
		return 0, exception.Internal(fmt.Errorf("insert payment: %w", err))
	}
	return paymentID, nil
}

func insertOrderRow(ctx context.Context, tx pgx.Tx, in entity.CreateOrderInput, paymentID, subtotal int64) (*entity.Order, error) {
	// order_number dibangkitkan di dalam SQL memakai sequence. Alternatifnya
	// (SELECT COUNT(*)+1) punya race condition: dua pesanan bersamaan bisa
	// mendapat nomor yang sama.
	const query = `
		INSERT INTO orders
			(order_number, table_id, customer_name, payment_id, source,
			 created_by_user_id, status, subtotal)
		VALUES (
			'ORD-' || to_char(now(), 'YYYYMMDD') || '-' ||
			         lpad(nextval('order_number_seq')::text, 6, '0'),
			$1, $2, $3, $4, $5, $6, $7
		)
		RETURNING ` + orderColumns

	order, err := scanOrder(tx.QueryRow(ctx, query,
		in.TableID,
		in.CustomerName,
		paymentID,
		in.Source,
		in.CreatedByUserID,
		config.OrderStatusPending,
		subtotal,
	))
	if err != nil {
		var pgErr *config.PgError
		if errors.As(err, &pgErr) && pgErr.Code == config.PgErrForeignKeyViolation {
			return nil, exception.BadRequest("ORDER_400", "table or user reference is invalid")
		}
		return nil, exception.Internal(fmt.Errorf("insert order: %w", err))
	}
	return &order, nil
}

// insertOrderItems menulis item dan add-on memakai pgx.Batch.
//
// Batch mengirim seluruh statement dalam satu perjalanan jaringan. Dengan
// pesanan 4 item + 6 add-on, ini memangkas 10 round trip jadi 2 — selisih
// yang menentukan apakah request masuk anggaran 20 ms atau tidak.
func insertOrderItems(ctx context.Context, tx pgx.Tx, orderID int64, items []pricedItem) error {
	const itemQuery = `
		INSERT INTO order_items
			(order_id, product_id, product_name, unit_price, quantity, subtotal, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	batch := &pgx.Batch{}
	for _, item := range items {
		batch.Queue(itemQuery,
			orderID, item.product.ID, item.product.Name,
			item.product.Price, item.quantity, item.subtotal, item.notes)
	}

	results := tx.SendBatch(ctx, batch)

	itemIDs := make([]int64, len(items))
	for i := range items {
		if err := results.QueryRow().Scan(&itemIDs[i]); err != nil {
			_ = results.Close()
			return exception.Internal(fmt.Errorf("insert order item: %w", err))
		}
	}
	if err := results.Close(); err != nil {
		return exception.Internal(fmt.Errorf("close item batch: %w", err))
	}

	for i := range items {
		items[i].persistedID = itemIDs[i]
	}

	// Add-on menyusul karena butuh ID item yang baru saja dibuat.
	const addonQuery = `
		INSERT INTO order_item_addons
			(order_item_id, product_addon_id, addon_name, addon_price, quantity, subtotal)
		VALUES ($1, $2, $3, $4, $5, $6)`

	addonBatch := &pgx.Batch{}
	count := 0
	for _, item := range items {
		for _, a := range item.addons {
			addonBatch.Queue(addonQuery,
				item.persistedID, a.addon.ID, a.addon.Name,
				a.addon.Price, a.quantity, a.subtotal)
			count++
		}
	}
	if count == 0 {
		return nil
	}

	addonResults := tx.SendBatch(ctx, addonBatch)
	for i := 0; i < count; i++ {
		if _, err := addonResults.Exec(); err != nil {
			_ = addonResults.Close()
			return exception.Internal(fmt.Errorf("insert order item addon: %w", err))
		}
	}
	if err := addonResults.Close(); err != nil {
		return exception.Internal(fmt.Errorf("close addon batch: %w", err))
	}
	return nil
}

// decreaseProductStock mengurangi stok produk.
//
// Syarat stock >= $1 diulang di WHERE sebagai jaring pengaman: baris sudah
// terkunci sejak lockOrderProducts, tapi kalau suatu saat lock itu hilang
// karena perubahan kode, pengurangan tetap tidak bisa membuat stok negatif.
func decreaseProductStock(ctx context.Context, tx pgx.Tx, items []pricedItem) error {
	const query = `
		UPDATE products
		SET stock = stock - $1, updated_at = now()
		WHERE id = $2 AND stock >= $1`

	needed := make(map[int64]int, len(items))
	for _, item := range items {
		needed[item.product.ID] += item.quantity
	}

	batch := &pgx.Batch{}
	for productID, qty := range needed {
		batch.Queue(query, qty, productID)
	}

	results := tx.SendBatch(ctx, batch)
	for range needed {
		tag, err := results.Exec()
		if err != nil {
			_ = results.Close()
			return exception.Internal(fmt.Errorf("decrease stock: %w", err))
		}
		if tag.RowsAffected() == 0 {
			_ = results.Close()
			return exception.Conflict("ORDER_409", "stock changed, please review your order")
		}
	}
	if err := results.Close(); err != nil {
		return exception.Internal(fmt.Errorf("close stock batch: %w", err))
	}
	return nil
}

// restoreProductStock mengembalikan stok dari seluruh item sebuah pesanan.
// Produk yang sudah dihapus (product_id NULL) dilewati.
func restoreProductStock(ctx context.Context, tx pgx.Tx, orderID int64) error {
	const query = `
		UPDATE products p
		SET stock = p.stock + agg.qty, updated_at = now()
		FROM (
			SELECT product_id, SUM(quantity) AS qty
			FROM order_items
			WHERE order_id = $1 AND product_id IS NOT NULL
			GROUP BY product_id
		) AS agg
		WHERE p.id = agg.product_id`

	if _, err := tx.Exec(ctx, query, orderID); err != nil {
		return exception.Internal(fmt.Errorf("restore stock: %w", err))
	}
	return nil
}

// ============================================================================
// Helper
// ============================================================================

// rowScanner menyatukan pgx.Row dan pgx.Rows supaya scanOrder bisa dipakai
// baik untuk QueryRow maupun perulangan Query.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanOrder(row rowScanner) (entity.Order, error) {
	var o entity.Order
	err := row.Scan(
		&o.ID, &o.OrderNumber, &o.TableID, &o.CustomerName, &o.PaymentID,
		&o.Source, &o.CreatedByUserID, &o.Status, &o.Subtotal,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, config.ErrNoRows) {
			return o, err // diterjemahkan pemanggil
		}
		return o, exception.Internal(fmt.Errorf("order scan: %w", err))
	}
	return o, nil
}

func toEntityOrderItems(orderID int64, items []pricedItem) []entity.OrderItem {
	out := make([]entity.OrderItem, 0, len(items))
	for _, item := range items {
		productID := item.product.ID

		addons := make([]entity.OrderItemAddon, 0, len(item.addons))
		for _, a := range item.addons {
			addonID := a.addon.ID
			addons = append(addons, entity.OrderItemAddon{
				OrderItemID:    item.persistedID,
				ProductAddonID: &addonID,
				AddonName:      a.addon.Name,
				AddonPrice:     a.addon.Price,
				Quantity:       a.quantity,
				Subtotal:       a.subtotal,
			})
		}

		out = append(out, entity.OrderItem{
			ID:          item.persistedID,
			OrderID:     orderID,
			ProductID:   &productID,
			ProductName: item.product.Name,
			UnitPrice:   item.product.Price,
			Quantity:    item.quantity,
			Subtotal:    item.subtotal,
			Notes:       item.notes,
			Addons:      addons,
		})
	}
	return out
}