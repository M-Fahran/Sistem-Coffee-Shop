package payment

import (
	"bytes"
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"coffeeshop/internal/config"
)

// Endpoint Midtrans Core API.
const (
	midtransSandboxURL    = "https://api.sandbox.midtrans.com/v2"
	midtransProductionURL = "https://api.midtrans.com/v2"
)

const (
	// midtransTimeout menjaga agar request pembuatan pesanan tidak
	// menggantung menunggu gateway. Kalau lewat, pesanan tetap tersimpan
	// dengan status pending dan pelanggan bisa mencoba bayar lagi.
	midtransTimeout = 10 * time.Second

	// midtransTimeLayout adalah format waktu di respons Midtrans.
	midtransTimeLayout = "2006-01-02 15:04:05"
)

// MidtransGateway berbicara dengan Midtrans Core API.
//
// Core API dipilih, bukan Snap, karena pelanggan sudah berada di halaman kita
// setelah scan QR meja. Snap akan melempar mereka ke halaman Midtrans;
// Core API memberi qr_string mentah supaya QR-nya dirender di halaman sendiri.
type MidtransGateway struct {
	serverKey string
	baseURL   string
	client    *http.Client
	location  *time.Location
}

// NewMidtransGateway membuat klien Midtrans.
//
// serverKey TIDAK BOLEH pernah muncul di log atau respons — dia dipakai
// sebagai kunci verifikasi signature, jadi bocornya setara dengan
// membiarkan orang memalsukan konfirmasi pembayaran.
func NewMidtransGateway(serverKey string, isProduction bool) *MidtransGateway {
	baseURL := midtransSandboxURL
	if isProduction {
		baseURL = midtransProductionURL
	}

	// Midtrans mengirim waktu tanpa zona, dalam WIB.
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	return &MidtransGateway{
		serverKey: serverKey,
		baseURL:   baseURL,
		client:    &http.Client{Timeout: midtransTimeout},
		location:  loc,
	}
}

func (g *MidtransGateway) Name() string { return "Midtrans" }

// ============================================================================
// Charge
// ============================================================================

type midtransChargeRequest struct {
	PaymentType        string                    `json:"payment_type"`
	TransactionDetails midtransTransactionDetail `json:"transaction_details"`
	CustomerDetails    *midtransCustomer         `json:"customer_details,omitempty"`
	ItemDetails        []midtransItem            `json:"item_details,omitempty"`
	QRIS               *midtransQRIS             `json:"qris,omitempty"`
	BankTransfer       *midtransBankTransfer     `json:"bank_transfer,omitempty"`
}

type midtransTransactionDetail struct {
	OrderID     string `json:"order_id"`
	GrossAmount int64  `json:"gross_amount"`
}

type midtransCustomer struct {
	FirstName string `json:"first_name,omitempty"`
	Email     string `json:"email,omitempty"`
}

type midtransItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
}

type midtransQRIS struct {
	Acquirer string `json:"acquirer"`
}

type midtransBankTransfer struct {
	Bank string `json:"bank"`
}

type midtransChargeResponse struct {
	StatusCode        string           `json:"status_code"`
	StatusMessage     string           `json:"status_message"`
	TransactionID     string           `json:"transaction_id"`
	OrderID           string           `json:"order_id"`
	GrossAmount       string           `json:"gross_amount"`
	TransactionStatus string           `json:"transaction_status"`
	ExpiryTime        string           `json:"expiry_time"`
	QRString          string           `json:"qr_string"`
	Actions           []midtransAction `json:"actions"`
	VANumbers         []midtransVA     `json:"va_numbers"`
}

type midtransAction struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Method string `json:"method"`
}

type midtransVA struct {
	Bank     string `json:"bank"`
	VANumber string `json:"va_number"`
}

// Charge membuat transaksi di Midtrans dan mengembalikan kode pembayaran.
func (g *MidtransGateway) Charge(ctx context.Context, req ChargeRequest) (*ChargeResult, error) {
	body, err := g.buildChargeRequest(req)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("midtrans encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/charge", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("midtrans build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", g.basicAuth())

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("midtrans call: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("midtrans read response: %w", err)
	}

	var parsed midtransChargeResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("midtrans decode response: %w", err)
	}

	// Midtrans memakai status_code di body, bukan HTTP status. 200/201 sukses.
	if parsed.StatusCode != "200" && parsed.StatusCode != "201" {
		return nil, fmt.Errorf("midtrans charge ditolak (%s): %s",
			parsed.StatusCode, parsed.StatusMessage)
	}

	return &ChargeResult{
		ExternalID:  parsed.TransactionID,
		PaymentCode: g.extractPaymentCode(parsed),
		ExpiresAt:   g.parseTime(parsed.ExpiryTime),
		RawResponse: string(respBody),
	}, nil
}

func (g *MidtransGateway) buildChargeRequest(req ChargeRequest) (*midtransChargeRequest, error) {
	out := &midtransChargeRequest{
		TransactionDetails: midtransTransactionDetail{
			OrderID:     req.PaymentRef,
			GrossAmount: req.Amount,
		},
	}

	if req.CustomerName != "" || req.CustomerEmail != "" {
		out.CustomerDetails = &midtransCustomer{
			FirstName: req.CustomerName,
			Email:     req.CustomerEmail,
		}
	}

	for _, item := range req.Items {
		out.ItemDetails = append(out.ItemDetails, midtransItem{
			ID:       item.ID,
			Name:     truncate(item.Name, 50), // batas Midtrans
			Price:    item.Price,
			Quantity: item.Quantity,
		})
	}

	switch req.PaymentMethod {
	case config.PaymentMethodQRIS:
		out.PaymentType = "qris"
		out.QRIS = &midtransQRIS{Acquirer: "gopay"}
	case config.PaymentMethodVA:
		out.PaymentType = "bank_transfer"
		out.BankTransfer = &midtransBankTransfer{Bank: "bca"}
	case config.PaymentMethodEWallet:
		out.PaymentType = "gopay"
	case config.PaymentMethodCash:
		// Tunai tidak pernah sampai ke gateway — kasir yang menerima uangnya.
		return nil, ErrUnsupportedMethod
	default:
		return nil, ErrUnsupportedMethod
	}

	return out, nil
}

// extractPaymentCode mengambil kode yang perlu ditampilkan ke pelanggan.
// Bentuknya berbeda per metode, jadi disamakan di sini supaya service tidak
// perlu tahu detail Midtrans.
func (g *MidtransGateway) extractPaymentCode(resp midtransChargeResponse) string {
	if resp.QRString != "" {
		return resp.QRString
	}
	if len(resp.VANumbers) > 0 {
		return resp.VANumbers[0].VANumber
	}
	for _, action := range resp.Actions {
		if action.Name == "generate-qr-code" || action.Name == "deeplink-redirect" {
			return action.URL
		}
	}
	return ""
}

// ============================================================================
// Webhook
// ============================================================================

type midtransNotification struct {
	TransactionTime   string `json:"transaction_time"`
	TransactionStatus string `json:"transaction_status"`
	TransactionID     string `json:"transaction_id"`
	StatusCode        string `json:"status_code"`
	SignatureKey      string `json:"signature_key"`
	SettlementTime    string `json:"settlement_time"`
	PaymentType       string `json:"payment_type"`
	OrderID           string `json:"order_id"`
	GrossAmount       string `json:"gross_amount"`
	FraudStatus       string `json:"fraud_status"`
}

// ParseWebhook memverifikasi dan menerjemahkan notifikasi Midtrans.
//
// Signature Midtrans dihitung sebagai:
//
//	SHA512(order_id + status_code + gross_amount + server_key)
//
// Tanpa verifikasi ini, siapa pun yang tahu URL webhook bisa mengirim
// "pembayaran lunas" palsu dan mendapat kopi gratis.
func (g *MidtransGateway) ParseWebhook(body []byte) (*WebhookEvent, error) {
	var n midtransNotification
	if err := json.Unmarshal(body, &n); err != nil {
		return nil, fmt.Errorf("midtrans decode notification: %w", err)
	}

	if err := g.verifySignature(n); err != nil {
		return nil, err
	}

	status, err := g.mapStatus(n)
	if err != nil {
		return nil, err
	}

	// gross_amount datang sebagai string desimal ("50000.00"). Dibaca sebagai
	// float lalu dibulatkan, karena rupiah tidak punya pecahan.
	amount, err := parseAmount(n.GrossAmount)
	if err != nil {
		return nil, fmt.Errorf("midtrans gross_amount tidak valid: %w", err)
	}

	var paidAt *time.Time
	if status == config.PaymentStatusPaid {
		if t := g.parseTime(n.SettlementTime); t != nil {
			paidAt = t
		} else if t := g.parseTime(n.TransactionTime); t != nil {
			paidAt = t
		} else {
			now := time.Now()
			paidAt = &now
		}
	}

	return &WebhookEvent{
		PaymentRef: n.OrderID,
		ExternalID: n.TransactionID,
		Status:     status,
		Amount:     amount,
		PaidAt:     paidAt,
		// EventKey menggabungkan transaksi dan statusnya: notifikasi ulang
		// dengan status sama akan tertolak, tapi perpindahan status
		// (pending → settlement) tetap diproses.
		EventKey: fmt.Sprintf("midtrans:%s:%s:%s", n.OrderID, n.TransactionID, n.TransactionStatus),
		RawBody:  string(body),
	}, nil
}

func (g *MidtransGateway) verifySignature(n midtransNotification) error {
	payload := n.OrderID + n.StatusCode + n.GrossAmount + g.serverKey
	sum := sha512.Sum512([]byte(payload))
	expected := hex.EncodeToString(sum[:])

	// subtle.ConstantTimeCompare, bukan ==, supaya lama perbandingan tidak
	// bergantung pada seberapa banyak karakter awal yang cocok.
	if subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(n.SignatureKey))) != 1 {
		return ErrInvalidSignature
	}
	return nil
}

// mapStatus menerjemahkan status Midtrans ke enum internal.
//
// Perhatikan penanganan "capture": statusnya belum tentu lunas — kalau
// fraud_status masih "challenge", dana ditahan dan menunggu keputusan manual.
// Memperlakukannya sebagai lunas berarti menyerahkan pesanan untuk pembayaran
// yang mungkin dibatalkan.
func (g *MidtransGateway) mapStatus(n midtransNotification) (config.PaymentStatus, error) {
	switch n.TransactionStatus {
	case "capture":
		if n.FraudStatus == "accept" {
			return config.PaymentStatusPaid, nil
		}
		if n.FraudStatus == "challenge" {
			return config.PaymentStatusPending, nil
		}
		return config.PaymentStatusFailed, nil

	case "settlement":
		return config.PaymentStatusPaid, nil

	case "pending":
		return config.PaymentStatusPending, nil

	case "deny", "cancel", "refund", "partial_refund", "chargeback", "partial_chargeback":
		return config.PaymentStatusFailed, nil

	case "expire", "failure":
		return config.PaymentStatusExpired, nil

	default:
		return 0, fmt.Errorf("%w: %q", ErrUnknownStatus, n.TransactionStatus)
	}
}

// ============================================================================
// Helper
// ============================================================================

func (g *MidtransGateway) basicAuth() string {
	// Midtrans memakai server key sebagai username, password kosong.
	encoded := base64.StdEncoding.EncodeToString([]byte(g.serverKey + ":"))
	return "Basic " + encoded
}

func (g *MidtransGateway) parseTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	t, err := time.ParseInLocation(midtransTimeLayout, value, g.location)
	if err != nil {
		return nil
	}
	return &t
}

func parseAmount(value string) (int64, error) {
	// Bentuknya "50000.00". Potong pecahannya alih-alih memakai float,
	// supaya tidak ada pembulatan yang mengubah nominal.
	if idx := strings.IndexByte(value, '.'); idx >= 0 {
		value = value[:idx]
	}
	return strconv.ParseInt(value, 10, 64)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}