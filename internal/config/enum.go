// enum.go — semua nilai enum aplikasi.
//
// Enum disimpan di database sebagai SMALLINT, tapi di API muncul sebagai
// string ("admin", "completed", dst) lewat MarshalJSON. Jadi angka hanya
// urusan internal; kontrak API tetap terbaca manusia.
//
// PENTING: nilai numerik di sini HARUS cocok dengan CHECK constraint di
// 000001_init_schema.up.sql. Kalau menambah nilai baru, ubah dua-duanya.
//
// Nilai 0 sengaja tidak pernah valid. Dengan begitu struct yang lupa diisi
// gagal saat ditulis ke database, alih-alih diam-diam bermakna nilai pertama.
package config

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// ============================================================================
// UserRole
// ============================================================================

type UserRole int16

const (
	UserRoleAdmin   UserRole = 1
	UserRoleCashier UserRole = 2
)

var (
	userRoleNames = map[UserRole]string{
		UserRoleAdmin:   "admin",
		UserRoleCashier: "cashier",
	}
	userRoleValues = invertEnum(userRoleNames)
)

func (r UserRole) String() string { return enumName(r, userRoleNames) }
func (r UserRole) Valid() bool    { return enumValid(r, userRoleNames) }

func (r UserRole) MarshalJSON() ([]byte, error) {
	return enumMarshal(r, userRoleNames, "user role")
}
func (r UserRole) Value() (driver.Value, error) {
	return enumValue(r, userRoleNames, "user role")
}
func (r *UserRole) UnmarshalJSON(b []byte) error {
	return enumUnmarshal(b, r, userRoleValues, "user role")
}
func (r *UserRole) Scan(src any) error {
	return enumScan(src, r, userRoleNames, "user role")
}

// ParseUserRole mengubah string jadi UserRole. Dipakai request binder.
func ParseUserRole(s string) (UserRole, error) {
	return enumParse(s, userRoleValues, "user role")
}

// ============================================================================
// PaymentMethod
// ============================================================================

type PaymentMethod int16

const (
	PaymentMethodQRIS    PaymentMethod = 1
	PaymentMethodVA      PaymentMethod = 2
	PaymentMethodEWallet PaymentMethod = 3
	PaymentMethodCash    PaymentMethod = 4
)

var (
	paymentMethodNames = map[PaymentMethod]string{
		PaymentMethodQRIS:    "qris",
		PaymentMethodVA:      "va",
		PaymentMethodEWallet: "ewallet",
		PaymentMethodCash:    "cash",
	}
	paymentMethodValues = invertEnum(paymentMethodNames)
)

func (m PaymentMethod) String() string { return enumName(m, paymentMethodNames) }
func (m PaymentMethod) Valid() bool    { return enumValid(m, paymentMethodNames) }

func (m PaymentMethod) MarshalJSON() ([]byte, error) {
	return enumMarshal(m, paymentMethodNames, "payment method")
}
func (m PaymentMethod) Value() (driver.Value, error) {
	return enumValue(m, paymentMethodNames, "payment method")
}
func (m *PaymentMethod) UnmarshalJSON(b []byte) error {
	return enumUnmarshal(b, m, paymentMethodValues, "payment method")
}
func (m *PaymentMethod) Scan(src any) error {
	return enumScan(src, m, paymentMethodNames, "payment method")
}

func ParsePaymentMethod(s string) (PaymentMethod, error) {
	return enumParse(s, paymentMethodValues, "payment method")
}

// IsOnline melaporkan apakah metode ini butuh payment gateway.
// Cash diselesaikan di kasir, sisanya lewat provider.
func (m PaymentMethod) IsOnline() bool { return m != PaymentMethodCash }

// ============================================================================
// PaymentStatus
// ============================================================================

type PaymentStatus int16

const (
	PaymentStatusPending PaymentStatus = 1
	PaymentStatusPaid    PaymentStatus = 2
	PaymentStatusFailed  PaymentStatus = 3
	PaymentStatusExpired PaymentStatus = 4
)

var (
	paymentStatusNames = map[PaymentStatus]string{
		PaymentStatusPending: "pending",
		PaymentStatusPaid:    "paid",
		PaymentStatusFailed:  "failed",
		PaymentStatusExpired: "expired",
	}
	paymentStatusValues = invertEnum(paymentStatusNames)
)

func (s PaymentStatus) String() string { return enumName(s, paymentStatusNames) }
func (s PaymentStatus) Valid() bool    { return enumValid(s, paymentStatusNames) }

func (s PaymentStatus) MarshalJSON() ([]byte, error) {
	return enumMarshal(s, paymentStatusNames, "payment status")
}
func (s PaymentStatus) Value() (driver.Value, error) {
	return enumValue(s, paymentStatusNames, "payment status")
}
func (s *PaymentStatus) UnmarshalJSON(b []byte) error {
	return enumUnmarshal(b, s, paymentStatusValues, "payment status")
}
func (s *PaymentStatus) Scan(src any) error {
	return enumScan(src, s, paymentStatusNames, "payment status")
}

func ParsePaymentStatus(str string) (PaymentStatus, error) {
	return enumParse(str, paymentStatusValues, "payment status")
}

// IsFinal melaporkan apakah status ini sudah tidak akan berubah lagi.
func (s PaymentStatus) IsFinal() bool { return s != PaymentStatusPending }

// ============================================================================
// OrderSource
// ============================================================================

type OrderSource int16

const (
	OrderSourceQR      OrderSource = 1
	OrderSourceCashier OrderSource = 2
)

var (
	orderSourceNames = map[OrderSource]string{
		OrderSourceQR:      "qr",
		OrderSourceCashier: "cashier",
	}
	orderSourceValues = invertEnum(orderSourceNames)
)

func (s OrderSource) String() string { return enumName(s, orderSourceNames) }
func (s OrderSource) Valid() bool    { return enumValid(s, orderSourceNames) }

func (s OrderSource) MarshalJSON() ([]byte, error) {
	return enumMarshal(s, orderSourceNames, "order source")
}
func (s OrderSource) Value() (driver.Value, error) {
	return enumValue(s, orderSourceNames, "order source")
}
func (s *OrderSource) UnmarshalJSON(b []byte) error {
	return enumUnmarshal(b, s, orderSourceValues, "order source")
}
func (s *OrderSource) Scan(src any) error {
	return enumScan(src, s, orderSourceNames, "order source")
}

func ParseOrderSource(str string) (OrderSource, error) {
	return enumParse(str, orderSourceValues, "order source")
}

// ============================================================================
// OrderStatus
// ============================================================================

type OrderStatus int16

const (
	OrderStatusPending   OrderStatus = 1
	OrderStatusConfirmed OrderStatus = 2
	OrderStatusPreparing OrderStatus = 3
	OrderStatusReady     OrderStatus = 4
	OrderStatusCompleted OrderStatus = 5
	OrderStatusCancelled OrderStatus = 6
)

var (
	orderStatusNames = map[OrderStatus]string{
		OrderStatusPending:   "pending",
		OrderStatusConfirmed: "confirmed",
		OrderStatusPreparing: "preparing",
		OrderStatusReady:     "ready",
		OrderStatusCompleted: "completed",
		OrderStatusCancelled: "cancelled",
	}
	orderStatusValues = invertEnum(orderStatusNames)
)

func (s OrderStatus) String() string { return enumName(s, orderStatusNames) }
func (s OrderStatus) Valid() bool    { return enumValid(s, orderStatusNames) }

func (s OrderStatus) MarshalJSON() ([]byte, error) {
	return enumMarshal(s, orderStatusNames, "order status")
}
func (s OrderStatus) Value() (driver.Value, error) {
	return enumValue(s, orderStatusNames, "order status")
}
func (s *OrderStatus) UnmarshalJSON(b []byte) error {
	return enumUnmarshal(b, s, orderStatusValues, "order status")
}
func (s *OrderStatus) Scan(src any) error {
	return enumScan(src, s, orderStatusNames, "order status")
}

func ParseOrderStatus(str string) (OrderStatus, error) {
	return enumParse(str, orderStatusValues, "order status")
}

// IsActive melaporkan apakah pesanan masih perlu ditangani dapur/kasir.
// Cocok dengan partial index idx_orders_active.
func (s OrderStatus) IsActive() bool {
	switch s {
	case OrderStatusPending, OrderStatusConfirmed, OrderStatusPreparing, OrderStatusReady:
		return true
	default:
		return false
	}
}

// CanTransitionTo menegakkan alur status yang sah. Tanpa ini, pesanan bisa
// meloncat dari "pending" langsung ke "completed" lewat request yang iseng.
func (s OrderStatus) CanTransitionTo(next OrderStatus) bool {
	allowed := map[OrderStatus][]OrderStatus{
		OrderStatusPending:   {OrderStatusConfirmed, OrderStatusCancelled},
		OrderStatusConfirmed: {OrderStatusPreparing, OrderStatusCancelled},
		OrderStatusPreparing: {OrderStatusReady, OrderStatusCancelled},
		OrderStatusReady:     {OrderStatusCompleted},
		OrderStatusCompleted: {},
		OrderStatusCancelled: {},
	}
	for _, candidate := range allowed[s] {
		if candidate == next {
			return true
		}
	}
	return false
}

// ============================================================================
// Helper generik
//
// Semua enum di atas int16 dan berperilaku identik, jadi logikanya ditulis
// sekali di sini. Nama helper diberi prefiks "enum" supaya tidak bentrok
// dengan identifier lain di package config.
//
// Menambah enum baru cukup: type + konstanta + map nama + enam method.
// ============================================================================

type enumType interface{ ~int16 }

func invertEnum[T enumType](names map[T]string) map[string]T {
	out := make(map[string]T, len(names))
	for v, n := range names {
		out[n] = v
	}
	return out
}

func enumName[T enumType](v T, names map[T]string) string {
	if n, ok := names[v]; ok {
		return n
	}
	return fmt.Sprintf("unknown(%d)", int16(v))
}

func enumValid[T enumType](v T, names map[T]string) bool {
	_, ok := names[v]
	return ok
}

func enumParse[T enumType](s string, values map[string]T, label string) (T, error) {
	v, ok := values[s]
	if !ok {
		return 0, fmt.Errorf("%s tidak dikenal: %q", label, s)
	}
	return v, nil
}

func enumMarshal[T enumType](v T, names map[T]string, label string) ([]byte, error) {
	n, ok := names[v]
	if !ok {
		return nil, fmt.Errorf("%s tidak valid: %d", label, int16(v))
	}
	return json.Marshal(n)
}

func enumUnmarshal[T enumType](b []byte, target *T, values map[string]T, label string) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("%s harus berupa string", label)
	}
	v, err := enumParse(s, values, label)
	if err != nil {
		return err
	}
	*target = v
	return nil
}

// enumValue dipakai saat menulis ke database. Nilai tidak valid ditolak di
// sini, sebelum sempat menyentuh CHECK constraint — error-nya jadi lebih jelas.
func enumValue[T enumType](v T, names map[T]string, label string) (driver.Value, error) {
	if _, ok := names[v]; !ok {
		return nil, fmt.Errorf("%s tidak valid: %d", label, int16(v))
	}
	return int64(v), nil
}

// enumScan dipakai saat membaca dari database. pgx mengirim SMALLINT sebagai
// int64, tapi tipe lain ikut ditangani supaya tidak rapuh.
func enumScan[T enumType](src any, target *T, names map[T]string, label string) error {
	var raw int64

	switch v := src.(type) {
	case int64:
		raw = v
	case int32:
		raw = int64(v)
	case int16:
		raw = int64(v)
	case int:
		raw = int64(v)
	case nil:
		return fmt.Errorf("%s: NULL tidak diperbolehkan", label)
	default:
		return fmt.Errorf("%s: tipe tidak didukung %T", label, src)
	}

	converted := T(raw)
	if _, ok := names[converted]; !ok {
		return fmt.Errorf("%s tidak dikenal di database: %d", label, raw)
	}
	*target = converted
	return nil
}