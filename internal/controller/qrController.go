package controller

import (
	"time"

	"github.com/gin-gonic/gin"

	tableReq "coffeeshop/internal/request/table"
	"coffeeshop/internal/service"
	"coffeeshop/internal/support/response"
)

type QRController struct {
	svc *service.TableService
}

func NewQRController(svc *service.TableService) *QRController {
	return &QRController{svc: svc}
}

// tableSessionResponse adalah satu-satunya bentuk data meja yang boleh
// dilihat pelanggan. Perhatikan tidak ada qr_token di sini — entity.Table
// punya field itu dengan tag JSON, jadi mengembalikan entity mentah ke
// endpoint publik sama saja membagikan kredensial meja.
type tableSessionResponse struct {
	SessionToken string          `json:"session_token"`
	ExpiresAt    time.Time       `json:"expires_at"`
	Table        publicTableView `json:"table"`
}

type publicTableView struct {
	ID     int64  `json:"id"`
	Number string `json:"number"`
}

// CreateSession menukar QR token meja dengan session token berumur pendek.
//
//	POST /api/v1/qr/:token/session
//
// Setelah dapat session token, frontend harus membuang QR token dari URL
// (history.replaceState) supaya tidak ikut terkirim di header Referer atau
// tersimpan di riwayat browser.
func (h *QRController) CreateSession(c *gin.Context) {
	qrToken, err := tableReq.BindQRTokenParam(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	sess, err := h.svc.CreateSession(c.Request.Context(), qrToken)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response.Created(c, "session created", tableSessionResponse{
		SessionToken: sess.Token,
		ExpiresAt:    sess.ExpiresAt,
		Table: publicTableView{
			ID:     sess.Table.ID,
			Number: sess.Table.Number,
		},
	})
}