package entity

import (
	"time"

	"coffeeshop/internal/config"
)

// ============================================================================
// Entity — bentuk data yang dibaca dari database dan dikirim ke klien
// ============================================================================

// Order merepresentasikan satu pesanan.
//
// Kolom nullable di database WAJIB pointer di sini. Sebelumnya TableID dan
// CreatedByUserID bertipe int64 non-pointer padahal kolomnya nullable —
// pgx langsung gagal scan begitu ketemu NULL.
type Order struct {
	ID           int64   `json:"id"`
	OrderNumber  string  `json:"order_number"`
	TableID      *int64  `json:"table_id"`      // NULL = take away
	CustomerName *string `json:"customer_name"` // NULL = tidak diisi

	// PaymentID NULL berarti pesanan dibuat tapi transaksi belum dibuka.
	PaymentID *int64 `json:"payment_id"`

	Source config.OrderSource `json:"source"`

	// CreatedByUserID NULL untuk pesanan dari QR — tidak ada kasir yang
	// mencatat. Wajib terisi untuk pesanan kasir (dijaga CHECK constraint).
	CreatedByUserID *int64 `json:"created_by_user_id"`

	Status config.OrderStatus `json:"status"`

	// Subtotal dalam rupiah penuh.
	Subtotal int64 `json:"subtotal"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Items []OrderItem `json:"items,omitempty"`
}

// OrderItem adalah satu baris produk dalam pesanan.
//
// ProductName dan UnitPrice sengaja disalin (bukan JOIN ke products), supaya
// struk lama tetap menampilkan harga saat itu meski produknya berubah.
type OrderItem struct {
	ID          int64   `json:"id"`
	OrderID     int64   `json:"order_id"`
	ProductID   *int64  `json:"product_id"` // NULL kalau produk sudah dihapus
	ProductName string  `json:"product_name"`
	UnitPrice   int64   `json:"unit_price"`
	Quantity    int     `json:"quantity"`
	Subtotal    int64   `json:"subtotal"`
	Notes       *string `json:"notes,omitempty"` // "es sedikit", "tanpa gula"

	Addons []OrderItemAddon `json:"addons,omitempty"`
}

// OrderItemAddon adalah tambahan pada satu item pesanan.
type OrderItemAddon struct {
	ID             int64  `json:"id"`
	OrderItemID    int64  `json:"order_item_id"`
	ProductAddonID *int64 `json:"product_addon_id"`
	AddonName      string `json:"addon_name"`
	AddonPrice     int64  `json:"addon_price"`
	Quantity       int    `json:"quantity"`
	Subtotal       int64  `json:"subtotal"`
}

// PaymentTransaction merepresentasikan satu percobaan pembayaran.
type PaymentTransaction struct {
	ID              int64                `json:"id"`
	PaymentRef      string               `json:"payment_ref"`
	ExternalID      *string              `json:"external_id"`
	Amount          int64                `json:"amount"`
	PaymentMethod   config.PaymentMethod `json:"payment_method"`
	Provider        string               `json:"provider"`
	Status          config.PaymentStatus `json:"status"`
	CustomerEmail   *string              `json:"customer_email"`
	HandledByUserID *int64               `json:"handled_by_user_id"`

	// Field berikut tidak pernah dikirim ke klien — hanya untuk audit.
	UserAgent *string `json:"-"`
	IPAddress *string `json:"-"`
	Payload   *string `json:"-"`

	PaidAt    *time.Time `json:"paid_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// ============================================================================
// Input — bentuk data yang masuk ke service, hasil terjemahan request
// ============================================================================

// CreateOrderInput adalah masukan pembuatan pesanan setelah lolos validasi
// request. Perhatikan TIDAK ADA field harga di sini — harga diambil dari
// database di dalam transaksi, supaya client tidak bisa menentukan sendiri
// berapa yang dia bayar.
type CreateOrderInput struct {
	// TableID diisi dari session QR atau pilihan kasir. NULL = take away.
	TableID *int64

	CustomerName  *string
	CustomerEmail *string

	Source config.OrderSource

	// CreatedByUserID wajib untuk Source == OrderSourceCashier
	// (ditegakkan CHECK constraint chk_cashier_order_has_creator).
	CreatedByUserID *int64

	PaymentMethod config.PaymentMethod

	// Provider diisi service: nama gateway untuk pembayaran online,
	// "Internal" untuk tunai.
	Provider string

	// Jejak audit, hanya disimpan — tidak pernah dikirim balik ke klien.
	UserAgent *string
	IPAddress *string

	Items []OrderItemInput
}

// OrderItemInput adalah satu baris keranjang.
type OrderItemInput struct {
	ProductID int64
	Quantity  int
	Notes     *string
	AddonIDs  []int64
}

// TotalQuantity dipakai untuk membatasi ukuran pesanan sebelum menyentuh
// database.
func (in CreateOrderInput) TotalQuantity() int {
	total := 0
	for _, item := range in.Items {
		total += item.Quantity
	}
	return total
}

// ProductIDs mengembalikan ID produk unik dalam pesanan.
func (in CreateOrderInput) ProductIDs() []int64 {
	seen := make(map[int64]struct{}, len(in.Items))
	ids := make([]int64, 0, len(in.Items))
	for _, item := range in.Items {
		if _, dup := seen[item.ProductID]; dup {
			continue
		}
		seen[item.ProductID] = struct{}{}
		ids = append(ids, item.ProductID)
	}
	return ids
}