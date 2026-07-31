package payment

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"coffeeshop/internal/config"
)

// FakeGateway adalah gateway tiruan untuk pengembangan lokal dan pengujian.
//
// Kenapa ini ada:
//
//   - order flow bisa dibangun dan diuji tanpa mendaftar ke provider mana pun
//   - test berjalan di CI tanpa memanggil layanan eksternal
//   - webhook bisa dipicu sendiri tanpa perlu tunnel (ngrok) ke localhost
//
// JANGAN dipakai di production. bootstrap harus menolak menyalakan ini kalau
// APP_ENV=production.
type FakeGateway struct {
	secret string
}

func NewFakeGateway(secret string) *FakeGateway {
	if secret == "" {
		secret = "fake-gateway-secret"
	}
	return &FakeGateway{secret: secret}
}

func (g *FakeGateway) Name() string { return "FakeGateway" }

func (g *FakeGateway) Charge(ctx context.Context, req ChargeRequest) (*ChargeResult, error) {
	if req.PaymentMethod == config.PaymentMethodCash {
		return nil, ErrUnsupportedMethod
	}

	externalID := "FAKE-" + randomHex(8)
	expiresAt := time.Now().Add(15 * time.Minute)

	// String QRIS palsu, formatnya mirip aslinya supaya frontend bisa diuji
	// merender QR code tanpa perlu transaksi sungguhan.
	code := fmt.Sprintf("00020101021226%s5204581453033605802ID6304%s",
		req.PaymentRef, randomHex(2))

	raw, _ := json.Marshal(map[string]any{
		"transaction_id": externalID,
		"order_id":       req.PaymentRef,
		"gross_amount":   req.Amount,
		"payment_type":   req.PaymentMethod.String(),
		"note":           "dibuat oleh FakeGateway, bukan transaksi sungguhan",
	})

	return &ChargeResult{
		ExternalID:  externalID,
		PaymentCode: code,
		ExpiresAt:   &expiresAt,
		RawResponse: string(raw),
	}, nil
}

type fakeNotification struct {
	PaymentRef  string `json:"payment_ref"`
	ExternalID  string `json:"external_id"`
	Status      string `json:"status"`
	GrossAmount int64  `json:"gross_amount"`
	Signature   string `json:"signature"`
}

// ParseWebhook memverifikasi notifikasi tiruan.
//
// Signature tetap diverifikasi meski ini gateway palsu — kalau jalur
// verifikasi cuma hidup di production, bug di situ baru ketahuan saat sudah
// ada uang sungguhan yang beredar.
func (g *FakeGateway) ParseWebhook(body []byte) (*WebhookEvent, error) {
	var n fakeNotification
	if err := json.Unmarshal(body, &n); err != nil {
		return nil, fmt.Errorf("fake gateway decode notification: %w", err)
	}

	expected := g.Sign(n.PaymentRef, n.Status, n.GrossAmount)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(n.Signature)) != 1 {
		return nil, ErrInvalidSignature
	}

	status, err := config.ParsePaymentStatus(n.Status)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrUnknownStatus, n.Status)
	}

	var paidAt *time.Time
	if status == config.PaymentStatusPaid {
		now := time.Now()
		paidAt = &now
	}

	return &WebhookEvent{
		PaymentRef: n.PaymentRef,
		ExternalID: n.ExternalID,
		Status:     status,
		Amount:     n.GrossAmount,
		PaidAt:     paidAt,
		EventKey:   fmt.Sprintf("fake:%s:%s", n.PaymentRef, n.Status),
		RawBody:    string(body),
	}, nil
}

// Sign membuat signature untuk notifikasi tiruan.
//
// Diekspor supaya endpoint pengembangan (POST /dev/simulate-payment) bisa
// menyusun notifikasi yang sah tanpa menduplikasi logikanya.
func (g *FakeGateway) Sign(paymentRef, status string, amount int64) string {
	mac := hmac.New(sha256.New, []byte(g.secret))
	fmt.Fprintf(mac, "%s|%s|%d", paymentRef, status, amount)
	return hex.EncodeToString(mac.Sum(nil))
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "0000000000000000"[:n*2]
	}
	return hex.EncodeToString(b)
}