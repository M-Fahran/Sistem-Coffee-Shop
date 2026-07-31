package controller

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"coffeeshop/internal/middleware"
	"coffeeshop/internal/service"
	"coffeeshop/internal/support/exception"
	"coffeeshop/internal/support/payment"
	"coffeeshop/internal/support/response"
)

// maxWebhookBody membatasi ukuran notifikasi. Body gateway hanya beberapa
// ratus byte; batas ini menutup upaya membanjiri memori lewat endpoint yang
// memang harus terbuka ke internet.
const maxWebhookBody = 64 << 10 // 64 KB

type PaymentController struct {
	svc  *service.PaymentService
	fake *payment.FakeGateway // nil di production
}

func NewPaymentController(svc *service.PaymentService, fake *payment.FakeGateway) *PaymentController {
	return &PaymentController{svc: svc, fake: fake}
}

// Webhook POST /api/v1/payments/webhook
//
// Endpoint ini TIDAK pakai middleware auth — gateway tidak punya token kita.
// Keamanannya sepenuhnya dari verifikasi signature di dalam service.
//
// Catatan penting soal status code: gateway mengulang notifikasi selama
// jawabannya bukan 2xx. Jadi 500 berarti "coba lagi nanti" (benar untuk
// gangguan sementara), sedangkan notifikasi yang memang tidak bisa diproses
// harus dijawab 200 supaya tidak diulang selamanya.
func (h *PaymentController) Webhook(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWebhookBody))
	if err != nil {
		_ = c.Error(exception.BadRequest("PAYMENT_400", "cannot read request body"))
		return
	}

	if err := h.svc.HandleWebhook(c.Request.Context(), body); err != nil {
		_ = c.Error(err)
		return
	}

	// Badan respons sengaja minimal — gateway tidak membacanya, dan
	// membocorkan detail internal di sini tidak ada gunanya.
	c.JSON(http.StatusOK, gin.H{"received": true})
}

// SettleCash POST /api/v1/orders/:id/settle-cash
//
// Dipakai kasir setelah menerima uang tunai. Hanya untuk pesanan bermetode
// tunai — pembayaran online ditolak di service, karena membiarkan kasir
// menandainya lunas secara manual membuka celah pesanan gratis.
func (h *PaymentController) SettleCash(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		_ = c.Error(exception.Unauthorized("AUTH_401", "authentication required"))
		return
	}

	orderID, err := orderIDParam(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if err := h.svc.SettleCash(c.Request.Context(), orderID, userID); err != nil {
		_ = c.Error(err)
		return
	}

	response.OK(c, "cash payment settled", nil)
}

// ============================================================================
// Simulator pengembangan
// ============================================================================

type simulatePaymentRequest struct {
	PaymentRef string `json:"payment_ref" binding:"required"`
	Status     string `json:"status"      binding:"required"`
	Amount     int64  `json:"amount"      binding:"required,gt=0"`
}

// SimulatePayment POST /dev/simulate-payment
//
// Memicu webhook tanpa gateway sungguhan, sehingga seluruh alur pembayaran
// bisa diuji di localhost tanpa ngrok.
//
// Rute ini HANYA didaftarkan saat APP_ENV bukan production — lihat routes.go.
// Kalau sampai hidup di production, siapa pun bisa menandai pesanannya lunas.
func (h *PaymentController) SimulatePayment(c *gin.Context) {
	if h.fake == nil {
		_ = c.Error(exception.NotFound("DEV_404", "simulator is not available"))
		return
	}

	var req simulatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(exception.Validation(err))
		return
	}

	// Notifikasi disusun lalu ditandatangani dengan mekanisme yang sama
	// seperti verifikasinya, jadi jalur signature ikut teruji.
	body, err := json.Marshal(map[string]any{
		"payment_ref":  req.PaymentRef,
		"external_id":  "SIM-" + req.PaymentRef,
		"status":       req.Status,
		"gross_amount": req.Amount,
		"signature":    h.fake.Sign(req.PaymentRef, req.Status, req.Amount),
	})
	if err != nil {
		_ = c.Error(exception.Internal(err))
		return
	}

	if err := h.svc.HandleWebhook(c.Request.Context(), body); err != nil {
		_ = c.Error(err)
		return
	}

	response.OK(c, "simulated webhook processed", gin.H{
		"payment_ref": req.PaymentRef,
		"status":      req.Status,
	})
}