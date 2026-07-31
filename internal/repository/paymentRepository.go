package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"coffeeshop/internal/config"
	"coffeeshop/internal/support/exception"
)

const paymentTxTimeout = 5 * time.Second

type PaymentRepository struct {
	db *config.Pool
}

func NewPaymentRepository(db *config.Pool) *PaymentRepository {
	return &PaymentRepository{db: db}
}

// ============================================================================
// Hasil charge
// ============================================================================

// AttachChargeResult menyimpan hasil pembuatan transaksi di gateway.
//
// Dijalankan SETELAH pesanan tersimpan, jadi kegagalan di sini tidak
// membatalkan pesanan — pelanggan tinggal mencoba bayar ulang.
func (r *PaymentRepository) AttachChargeResult(
	ctx context.Context,
	paymentID int64,
	externalID, paymentCode, rawResponse string,
	expiresAt *time.Time,
) error {
	const query = `
		UPDATE payment_transactions
		SET external_id = $1, payment_code = $2, payload = $3, expires_at = $4
		WHERE id = $5 AND status = $6`

	tag, err := r.db.Exec(ctx, query,
		externalID, paymentCode, rawResponse, expiresAt,
		paymentID, config.PaymentStatusPending)
	if err != nil {
		return exception.Internal(fmt.Errorf("attach charge result: %w", err))
	}
	if tag.RowsAffected() == 0 {
		// Statusnya sudah berubah — webhook mendahului respons charge.
		// Bukan kesalahan; data dari webhook lebih baru dan tidak boleh ditimpa.
		return nil
	}
	return nil
}

// GetChargeContext mengambil data yang dibutuhkan untuk memanggil gateway.
//
// Nominal diambil dari database, bukan dihitung ulang atau diterima dari
// klien — ini titik terakhir sebelum angka dikirim ke penyedia pembayaran.
func (r *PaymentRepository) GetChargeContext(
	ctx context.Context,
	paymentID int64,
) (paymentRef string, method config.PaymentMethod, amount int64, err error) {
	const query = `
		SELECT payment_ref, payment_method, amount
		FROM payment_transactions
		WHERE id = $1`

	err = r.db.QueryRow(ctx, query, paymentID).Scan(&paymentRef, &method, &amount)
	if err != nil {
		if errors.Is(err, config.ErrNoRows) {
			return "", 0, 0, exception.NotFound("PAYMENT_404", "payment not found")
		}
		return "", 0, 0, exception.Internal(fmt.Errorf("get charge context: %w", err))
	}
	return paymentRef, method, amount, nil
}

// ============================================================================
// Pembayaran tunai
// ============================================================================

// CashSettlement adalah hasil penyelesaian pembayaran tunai.
type CashSettlement struct {
	OrderID     int64
	OrderNumber string
	PaymentRef  string
}

// SettleCash menandai pembayaran tunai lunas dan mengantrekan invoice.
//
// Alurnya sama persis dengan webhook — kunci baris, cek status, ubah
// pembayaran dan pesanan, antrekan invoice, semuanya dalam satu transaksi.
// Bedanya hanya tidak ada signature yang perlu diverifikasi.
func (r *PaymentRepository) SettleCash(
	ctx context.Context,
	orderID, cashierUserID int64,
) (*CashSettlement, error) {
	ctx, cancel := context.WithTimeout(ctx, paymentTxTimeout)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("cash tx begin: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const lockQuery = `
		SELECT pt.id, pt.payment_ref, pt.amount, pt.status, pt.payment_method,
		       pt.customer_email, o.id, o.order_number
		FROM payment_transactions pt
		JOIN orders o ON o.payment_id = pt.id
		WHERE o.id = $1
		FOR UPDATE OF pt`

	var (
		p      lockedPayment
		method config.PaymentMethod
	)
	err = tx.QueryRow(ctx, lockQuery, orderID).Scan(
		&p.ID, &p.PaymentRef, &p.Amount, &p.Status, &method,
		&p.CustomerEmail, &p.OrderID, &p.OrderNumber,
	)
	if err != nil {
		if errors.Is(err, config.ErrNoRows) {
			return nil, exception.NotFound("ORDER_404", "order not found")
		}
		return nil, exception.Internal(fmt.Errorf("lock cash payment: %w", err))
	}

	// Pembayaran online diselesaikan gateway lewat webhook. Membiarkan kasir
	// menandainya lunas secara manual membuka celah kopi gratis.
	if method.IsOnline() {
		return nil, exception.Conflict("PAYMENT_409",
			"this order is paid through a payment provider")
	}
	if p.Status.IsFinal() {
		return nil, exception.Conflict("PAYMENT_409",
			fmt.Sprintf("payment is already %s", p.Status.String()))
	}

	const updateQuery = `
		UPDATE payment_transactions
		SET status = $1, paid_at = now(), handled_by_user_id = $2
		WHERE id = $3`

	if _, err := tx.Exec(ctx, updateQuery, config.PaymentStatusPaid, cashierUserID, p.ID); err != nil {
		return nil, exception.Internal(fmt.Errorf("settle cash: %w", err))
	}

	if err := applyOrderStatus(ctx, tx, p.OrderID, config.PaymentStatusPaid); err != nil {
		return nil, err
	}

	if p.CustomerEmail != nil {
		payload := InvoiceJobPayload{
			OrderID:       p.OrderID,
			OrderNumber:   p.OrderNumber,
			PaymentRef:    p.PaymentRef,
			CustomerEmail: *p.CustomerEmail,
		}
		if err := enqueueJob(ctx, tx, config.JobTypeInvoiceEmail, "invoice:"+p.PaymentRef, payload); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, exception.Internal(fmt.Errorf("cash tx commit: %w", err))
	}

	return &CashSettlement{
		OrderID:     p.OrderID,
		OrderNumber: p.OrderNumber,
		PaymentRef:  p.PaymentRef,
	}, nil
}

// ============================================================================
// Webhook
// ============================================================================

// WebhookResult melaporkan apa yang terjadi pada satu notifikasi.
type WebhookResult struct {
	// Duplicate berarti notifikasi ini sudah pernah diproses.
	Duplicate bool

	// Applied berarti status pembayaran benar-benar berubah.
	Applied bool

	PaymentRef  string
	OrderID     int64
	OrderNumber string
	NewStatus   config.PaymentStatus
}

// ApplyWebhook memproses satu notifikasi pembayaran secara atomik.
//
// Urutan langkahnya penting, dan setiap langkah menutup satu celah:
//
//  1. Catat event (UNIQUE)      → notifikasi kembar tertolak database
//  2. Kunci baris pembayaran    → dua notifikasi bersamaan tidak balapan
//  3. Cocokkan nominal          → notifikasi valid tapi nominalnya beda ditolak
//  4. Tolak status yang mundur  → "pending" susulan tidak membatalkan "paid"
//  5. Update pembayaran + order → satu commit
//  6. Antrekan invoice          → di transaksi yang SAMA, bukan setelahnya
//
// Langkah 6 itu inti pola outbox: kalau job ditulis ke Redis setelah commit
// dan proses mati di antaranya, invoice hilang tanpa jejak.
func (r *PaymentRepository) ApplyWebhook(
	ctx context.Context,
	provider string,
	event PaymentWebhookInput,
) (*WebhookResult, error) {
	ctx, cancel := context.WithTimeout(ctx, paymentTxTimeout)
	defer cancel()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("webhook tx begin: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Dedup lewat UNIQUE constraint.
	inserted, err := recordWebhookEvent(ctx, tx, provider, event)
	if err != nil {
		return nil, err
	}
	if !inserted {
		return &WebhookResult{Duplicate: true, PaymentRef: event.PaymentRef}, nil
	}

	// 2. Kunci baris pembayaran.
	current, err := lockPaymentByRef(ctx, tx, event.PaymentRef)
	if err != nil {
		return nil, err
	}

	// 3. Nominal wajib cocok. Signature yang sah pun tidak menjamin nominalnya
	//    benar — notifikasi bisa saja hasil replay transaksi lain.
	if current.Amount != event.Amount {
		return nil, exception.BadRequest("PAYMENT_400",
			fmt.Sprintf("amount mismatch for %s", event.PaymentRef))
	}

	// 4. Status final tidak boleh mundur. Midtrans bisa mengirim "pending"
	//    setelah "settlement" karena antrean notifikasi tidak berurutan.
	if current.Status.IsFinal() {
		return &WebhookResult{
			Duplicate:   true,
			PaymentRef:  event.PaymentRef,
			OrderID:     current.OrderID,
			OrderNumber: current.OrderNumber,
			NewStatus:   current.Status,
		}, nil
	}
	if current.Status == event.Status {
		return &WebhookResult{
			Duplicate:   true,
			PaymentRef:  event.PaymentRef,
			OrderID:     current.OrderID,
			OrderNumber: current.OrderNumber,
			NewStatus:   current.Status,
		}, nil
	}

	// 5. Terapkan perubahan.
	if err := updatePaymentStatus(ctx, tx, current.ID, event); err != nil {
		return nil, err
	}
	if err := applyOrderStatus(ctx, tx, current.OrderID, event.Status); err != nil {
		return nil, err
	}

	// 6. Antrekan invoice — hanya untuk pembayaran lunas yang punya email.
	if event.Status == config.PaymentStatusPaid && current.CustomerEmail != nil {
		payload := InvoiceJobPayload{
			OrderID:       current.OrderID,
			OrderNumber:   current.OrderNumber,
			PaymentRef:    current.PaymentRef,
			CustomerEmail: *current.CustomerEmail,
		}
		dedupKey := "invoice:" + current.PaymentRef

		if err := enqueueJob(ctx, tx, config.JobTypeInvoiceEmail, dedupKey, payload); err != nil {
			return nil, err
		}
	}

	if err := markWebhookProcessed(ctx, tx, event.EventKey); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, exception.Internal(fmt.Errorf("webhook tx commit: %w", err))
	}

	return &WebhookResult{
		Applied:     true,
		PaymentRef:  event.PaymentRef,
		OrderID:     current.OrderID,
		OrderNumber: current.OrderNumber,
		NewStatus:   event.Status,
	}, nil
}

// PaymentWebhookInput adalah notifikasi yang sudah diverifikasi gateway.
type PaymentWebhookInput struct {
	PaymentRef string
	ExternalID string
	Status     config.PaymentStatus
	Amount     int64
	PaidAt     *time.Time
	EventKey   string
	RawBody    string
}

// InvoiceJobPayload adalah isi job pengiriman invoice.
//
// Hanya berisi rujukan, bukan salinan data pesanan. Worker mengambil isi
// terbaru dari database saat menjalankan job — kalau data disalin ke payload,
// invoice bisa memuat informasi yang sudah basi.
type InvoiceJobPayload struct {
	OrderID       int64  `json:"order_id"`
	OrderNumber   string `json:"order_number"`
	PaymentRef    string `json:"payment_ref"`
	CustomerEmail string `json:"customer_email"`
}

// ============================================================================
// Langkah internal
// ============================================================================

type lockedPayment struct {
	ID            int64
	PaymentRef    string
	Amount        int64
	Status        config.PaymentStatus
	CustomerEmail *string
	OrderID       int64
	OrderNumber   string
}

// recordWebhookEvent mencatat notifikasi mentah.
//
// ON CONFLICT DO NOTHING membuat dedup ditegakkan database, bukan aplikasi.
// Dua instance server yang menerima notifikasi sama secara bersamaan tetap
// hanya menghasilkan satu pemrosesan.
func recordWebhookEvent(ctx context.Context, tx pgx.Tx, provider string, e PaymentWebhookInput) (bool, error) {
	const query = `
		INSERT INTO webhook_events (provider, event_key, payment_ref, raw_body)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (event_key) DO NOTHING`

	tag, err := tx.Exec(ctx, query, provider, e.EventKey, e.PaymentRef, e.RawBody)
	if err != nil {
		return false, exception.Internal(fmt.Errorf("record webhook event: %w", err))
	}
	return tag.RowsAffected() > 0, nil
}

func markWebhookProcessed(ctx context.Context, tx pgx.Tx, eventKey string) error {
	const query = `UPDATE webhook_events SET processed_at = now() WHERE event_key = $1`
	if _, err := tx.Exec(ctx, query, eventKey); err != nil {
		return exception.Internal(fmt.Errorf("mark webhook processed: %w", err))
	}
	return nil
}

// lockPaymentByRef mengunci baris pembayaran beserta pesanannya.
//
// FOR UPDATE OF pt membatasi lock hanya ke tabel pembayaran; baris orders
// ikut terbaca tapi tidak dikunci, supaya kasir tetap bisa memindahkan status
// pesanan lain saat webhook diproses.
func lockPaymentByRef(ctx context.Context, tx pgx.Tx, paymentRef string) (*lockedPayment, error) {
	const query = `
		SELECT pt.id, pt.payment_ref, pt.amount, pt.status, pt.customer_email,
		       o.id, o.order_number
		FROM payment_transactions pt
		JOIN orders o ON o.payment_id = pt.id
		WHERE pt.payment_ref = $1
		FOR UPDATE OF pt`

	var p lockedPayment
	err := tx.QueryRow(ctx, query, paymentRef).Scan(
		&p.ID, &p.PaymentRef, &p.Amount, &p.Status, &p.CustomerEmail,
		&p.OrderID, &p.OrderNumber,
	)
	if err != nil {
		if errors.Is(err, config.ErrNoRows) {
			return nil, exception.NotFound("PAYMENT_404", "payment not found")
		}
		return nil, exception.Internal(fmt.Errorf("lock payment: %w", err))
	}
	return &p, nil
}

func updatePaymentStatus(ctx context.Context, tx pgx.Tx, paymentID int64, e PaymentWebhookInput) error {
	// COALESCE menjaga external_id yang sudah ada: notifikasi susulan kadang
	// tidak menyertakannya, dan menimpanya dengan string kosong akan
	// memutus jejak rekonsiliasi.
	const query = `
		UPDATE payment_transactions
		SET status      = $1,
		    paid_at     = $2,
		    external_id = COALESCE(NULLIF($3, ''), external_id),
		    payload     = $4
		WHERE id = $5`

	if _, err := tx.Exec(ctx, query, e.Status, e.PaidAt, e.ExternalID, e.RawBody, paymentID); err != nil {
		return exception.Internal(fmt.Errorf("update payment status: %w", err))
	}
	return nil
}

// applyOrderStatus memindahkan status pesanan mengikuti hasil pembayaran.
//
// Lunas → confirmed (dapur boleh mulai). Gagal/kedaluwarsa → cancelled,
// sekaligus mengembalikan stok.
func applyOrderStatus(ctx context.Context, tx pgx.Tx, orderID int64, paymentStatus config.PaymentStatus) error {
	var next config.OrderStatus

	switch paymentStatus {
	case config.PaymentStatusPaid:
		next = config.OrderStatusConfirmed
	case config.PaymentStatusFailed, config.PaymentStatusExpired:
		next = config.OrderStatusCancelled
	default:
		return nil // pending tidak mengubah apa pun
	}

	var current config.OrderStatus
	err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&current)
	if err != nil {
		return exception.Internal(fmt.Errorf("lock order for payment: %w", err))
	}

	// Kasir bisa saja sudah memindahkan pesanan lebih jauh (misal langsung
	// ke "preparing"). Transisi yang tidak sah dilewati, bukan dipaksakan.
	if !current.CanTransitionTo(next) {
		return nil
	}

	if next == config.OrderStatusCancelled {
		if err := restoreProductStock(ctx, tx, orderID); err != nil {
			return err
		}
	}

	const query = `UPDATE orders SET status = $1, updated_at = now() WHERE id = $2`
	if _, err := tx.Exec(ctx, query, next, orderID); err != nil {
		return exception.Internal(fmt.Errorf("apply order status: %w", err))
	}
	return nil
}

// enqueueJob menulis satu pekerjaan ke outbox.
//
// ON CONFLICT DO NOTHING pada dedup_key mencegah invoice ganda kalau
// notifikasi lunas entah bagaimana lolos sampai sini dua kali.
func enqueueJob(ctx context.Context, tx pgx.Tx, jobType config.JobType, dedupKey string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return exception.Internal(fmt.Errorf("encode job payload: %w", err))
	}

	const query = `
		INSERT INTO outbox_jobs (job_type, payload, dedup_key)
		VALUES ($1, $2, $3)
		ON CONFLICT (dedup_key) DO NOTHING`

	if _, err := tx.Exec(ctx, query, jobType, raw, dedupKey); err != nil {
		return exception.Internal(fmt.Errorf("enqueue job: %w", err))
	}
	return nil
}