package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"coffeeshop/internal/config"
	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/support/exception"
	"coffeeshop/internal/support/payment"
)

type PaymentService struct {
	repo    *repository.PaymentRepository
	orders  *repository.OrdersRepository
	gateway payment.Gateway
	log     *slog.Logger
}

func NewPaymentService(
	repo *repository.PaymentRepository,
	orders *repository.OrdersRepository,
	gateway payment.Gateway,
	log *slog.Logger,
) *PaymentService {
	return &PaymentService{repo: repo, orders: orders, gateway: gateway, log: log}
}

// PaymentInstruction adalah yang ditampilkan ke pelanggan agar bisa membayar.
type PaymentInstruction struct {
	PaymentRef  string               `json:"payment_ref"`
	Method      config.PaymentMethod `json:"payment_method"`
	Amount      int64                `json:"amount"`
	PaymentCode string               `json:"payment_code,omitempty"`
	ExpiresAt   *string              `json:"expires_at,omitempty"`
}

// ============================================================================
// Charge
// ============================================================================

// Charge membuat transaksi di gateway untuk pesanan yang baru dibuat.
//
// Dipanggil SETELAH pesanan tersimpan, bukan di dalam transaksi pembuatan
// pesanan. Alasannya: panggilan HTTP ke gateway bisa memakan detik, dan
// memegang lock baris produk selama itu akan memblokir semua pesanan lain
// untuk produk yang sama.
//
// Kalau gateway gagal, pesanan tetap ada dengan status pending dan pelanggan
// bisa mencoba bayar ulang — jauh lebih baik daripada pesanan hilang.
func (s *PaymentService) Charge(
	ctx context.Context,
	order *entity.Order,
	customerEmail string,
) (*PaymentInstruction, error) {
	if order.PaymentID == nil {
		return nil, exception.Internal(fmt.Errorf("order %d tidak punya payment", order.ID))
	}

	paymentRef, method, amount, err := s.repo.GetChargeContext(ctx, *order.PaymentID)
	if err != nil {
		return nil, err
	}

	instruction := &PaymentInstruction{
		PaymentRef: paymentRef,
		Method:     method,
		Amount:     amount,
	}

	// Tunai tidak menyentuh gateway sama sekali — kasir yang menerima uangnya
	// dan menandai lunas lewat endpoint terpisah.
	if !method.IsOnline() {
		return instruction, nil
	}

	req := payment.ChargeRequest{
		PaymentRef:    paymentRef,
		Amount:        amount,
		PaymentMethod: method,
		CustomerEmail: customerEmail,
		Items:         toChargeItems(order.Items),
	}
	if order.CustomerName != nil {
		req.CustomerName = *order.CustomerName
	}

	result, err := s.gateway.Charge(ctx, req)
	if err != nil {
		if errors.Is(err, payment.ErrUnsupportedMethod) {
			return nil, exception.BadRequest("PAYMENT_400", "payment method is not supported")
		}
		// Pesanan tetap valid. Kembalikan 502 supaya klien tahu ini masalah
		// pihak ketiga, bukan kesalahan input mereka.
		s.log.Error("gateway charge failed",
			"order_number", order.OrderNumber, "payment_ref", paymentRef, "err", err)
		return nil, exception.New(502, "PAYMENT_502",
			"payment provider is unavailable, please try again")
	}

	if err := s.repo.AttachChargeResult(ctx, *order.PaymentID,
		result.ExternalID, result.PaymentCode, result.RawResponse, result.ExpiresAt); err != nil {
		// Transaksi sudah terbuat di gateway. Menggagalkan request di sini
		// justru menyesatkan — pelanggan tetap bisa membayar, dan webhook
		// akan menyelaraskan datanya nanti.
		s.log.Error("attach charge result failed",
			"payment_ref", paymentRef, "external_id", result.ExternalID, "err", err)
	}

	instruction.PaymentCode = result.PaymentCode
	if result.ExpiresAt != nil {
		formatted := result.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
		instruction.ExpiresAt = &formatted
	}
	return instruction, nil
}

// ============================================================================
// Webhook
// ============================================================================

// HandleWebhook memverifikasi lalu menerapkan notifikasi pembayaran.
//
// Verifikasi signature terjadi di dalam gateway.ParseWebhook, jadi tidak
// mungkin terlewat di sini.
func (s *PaymentService) HandleWebhook(ctx context.Context, body []byte) error {
	event, err := s.gateway.ParseWebhook(body)
	if err != nil {
		switch {
		case errors.Is(err, payment.ErrInvalidSignature):
			// Ada yang mencoba memalsukan pembayaran. Dicatat sebagai error
			// supaya muncul di monitoring.
			s.log.Error("webhook signature invalid", "provider", s.gateway.Name())
			return exception.Unauthorized("PAYMENT_401", "invalid signature")

		case errors.Is(err, payment.ErrUnknownStatus):
			// Status yang belum kita tangani. Bukan kegagalan — cukup
			// diabaikan, dan gateway tidak perlu mengirim ulang.
			s.log.Warn("webhook status not handled", "err", err)
			return nil

		default:
			return exception.BadRequest("PAYMENT_400", "malformed notification")
		}
	}

	result, err := s.repo.ApplyWebhook(ctx, s.gateway.Name(), repository.PaymentWebhookInput{
		PaymentRef: event.PaymentRef,
		ExternalID: event.ExternalID,
		Status:     event.Status,
		Amount:     event.Amount,
		PaidAt:     event.PaidAt,
		EventKey:   event.EventKey,
		RawBody:    event.RawBody,
	})
	if err != nil {
		return err
	}

	if result.Duplicate {
		s.log.Info("webhook ignored (duplicate)",
			"payment_ref", result.PaymentRef, "status", event.Status.String())
		return nil
	}

	s.log.Info("payment status applied",
		"payment_ref", result.PaymentRef,
		"order_number", result.OrderNumber,
		"status", result.NewStatus.String(),
	)
	return nil
}

// ============================================================================
// Pembayaran tunai
// ============================================================================

// SettleCash menandai pembayaran tunai sebagai lunas.
//
// Jalur ini tidak lewat gateway, jadi tidak ada signature yang bisa
// diverifikasi — pengamanannya ada di lapisan rute (hanya staff terautentikasi)
// dan pencatatan siapa yang menerima uangnya.
func (s *PaymentService) SettleCash(ctx context.Context, orderID, cashierUserID int64) error {
	result, err := s.repo.SettleCash(ctx, orderID, cashierUserID)
	if err != nil {
		return err
	}

	s.log.Info("cash payment settled",
		"order_number", result.OrderNumber,
		"payment_ref", result.PaymentRef,
		"by_user_id", cashierUserID,
	)
	return nil
}

// ============================================================================
// Helper
// ============================================================================

func toChargeItems(items []entity.OrderItem) []payment.ChargeItem {
	out := make([]payment.ChargeItem, 0, len(items))
	for _, item := range items {
		out = append(out, payment.ChargeItem{
			ID:       strconv.FormatInt(item.ID, 10),
			Name:     item.ProductName,
			Price:    item.UnitPrice,
			Quantity: item.Quantity,
		})

		// Add-on dikirim sebagai baris terpisah supaya total item_details
		// cocok dengan gross_amount — Midtrans menolak charge kalau
		// jumlahnya tidak sama persis.
		for _, addon := range item.Addons {
			out = append(out, payment.ChargeItem{
				ID:       fmt.Sprintf("%d-a%d", item.ID, addon.ID),
				Name:     "+ " + addon.AddonName,
				Price:    addon.AddonPrice,
				Quantity: addon.Quantity,
			})
		}
	}
	return out
}