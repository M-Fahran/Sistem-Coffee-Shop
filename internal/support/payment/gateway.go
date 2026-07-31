// Package payment menyediakan abstraksi payment gateway.
//
// Service TIDAK BOLEH mengimpor SDK Midtrans/Xendit secara langsung. Semua
// interaksi lewat interface Gateway di sini, supaya:
//
//   - order flow bisa diuji tanpa memanggil layanan eksternal
//   - pindah provider cukup mengganti satu implementasi
//   - pengembangan lokal jalan tanpa internet (pakai FakeGateway)
package payment

import (
	"context"
	"errors"
	"time"

	"coffeeshop/internal/config"
)

var (
	// ErrInvalidSignature berarti notifikasi tidak berasal dari gateway.
	// Request seperti ini harus ditolak dan dicatat — ada yang mencoba
	// memalsukan pembayaran.
	ErrInvalidSignature = errors.New("signature webhook tidak valid")

	// ErrUnsupportedMethod berarti metode pembayaran tidak didukung provider.
	ErrUnsupportedMethod = errors.New("metode pembayaran tidak didukung provider ini")

	// ErrUnknownStatus berarti gateway mengirim status yang belum dikenali.
	// Bukan kegagalan — hanya perlu diabaikan dan dicatat.
	ErrUnknownStatus = errors.New("status transaksi tidak dikenal")
)

// ChargeRequest adalah permintaan pembuatan transaksi ke gateway.
type ChargeRequest struct {
	// PaymentRef dikirim sebagai order_id ke gateway. Kita pakai referensi
	// sendiri, bukan ID pesanan, supaya percobaan bayar ulang atas satu
	// pesanan tetap punya identitas berbeda di sisi gateway.
	PaymentRef string

	Amount        int64
	PaymentMethod config.PaymentMethod

	CustomerName  string
	CustomerEmail string

	// Items hanya untuk ditampilkan di halaman gateway. Nominal yang mengikat
	// tetap Amount di atas.
	Items []ChargeItem
}

type ChargeItem struct {
	ID       string
	Name     string
	Price    int64
	Quantity int
}

// ChargeResult adalah hasil pembuatan transaksi.
type ChargeResult struct {
	// ExternalID adalah ID transaksi milik gateway, dipakai untuk rekonsiliasi.
	ExternalID string

	// PaymentCode berisi QR string (QRIS), nomor VA, atau deeplink e-wallet.
	// Frontend merender ini jadi QR code di halaman sendiri.
	PaymentCode string

	// ExpiresAt adalah batas waktu pembayaran menurut gateway.
	ExpiresAt *time.Time

	// RawResponse disimpan apa adanya untuk keperluan audit dan sengketa.
	RawResponse string
}

// WebhookEvent adalah notifikasi gateway yang sudah diterjemahkan.
type WebhookEvent struct {
	// PaymentRef mencocokkan notifikasi dengan baris payment_transactions.
	PaymentRef string

	ExternalID string
	Status     config.PaymentStatus

	// Amount dari notifikasi. WAJIB dicocokkan dengan nominal di database
	// sebelum status diubah — notifikasi yang lolos signature pun bisa saja
	// hasil replay transaksi lain.
	Amount int64

	PaidAt *time.Time

	// EventKey membedakan satu notifikasi dari notifikasi lain untuk
	// transaksi yang sama. Dipakai sebagai kunci dedup.
	EventKey string

	RawBody string
}

// Gateway adalah kontrak yang harus dipenuhi setiap provider.
type Gateway interface {
	// Name mengembalikan nama provider, disimpan di kolom provider.
	Name() string

	// Charge membuat transaksi baru di sisi gateway.
	Charge(ctx context.Context, req ChargeRequest) (*ChargeResult, error)

	// ParseWebhook memverifikasi keaslian notifikasi lalu menerjemahkannya.
	//
	// Verifikasi signature WAJIB dilakukan di dalam method ini, bukan
	// diserahkan ke pemanggil — supaya tidak mungkin terlewat.
	ParseWebhook(body []byte) (*WebhookEvent, error)
}