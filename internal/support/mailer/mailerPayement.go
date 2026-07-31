// Package mailer mengirim email transaksional.
//
// Sengaja dibuat sebagai interface: pengembangan lokal memakai LogMailer
// (cukup mencetak ke log), production memakai SMTPMailer. Tidak ada kode
// pemanggil yang perlu tahu bedanya.
package mailer

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/smtp"
	"strings"
	"time"
)

// Message adalah satu email siap kirim.
type Message struct {
	To      string
	Subject string
	HTML    string
}

// Mailer adalah kontrak pengiriman email.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// ============================================================================
// SMTP
// ============================================================================

type SMTPConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromName  string
	FromEmail string
}

type SMTPMailer struct {
	cfg SMTPConfig
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}

// Send mengirim satu email lewat SMTP.
//
// Header To dan Subject dibersihkan dari CR/LF sebelum disusun. Tanpa itu,
// alamat email berisi "\r\nBcc: ..." bisa menyisipkan header tambahan —
// pelanggan menentukan sendiri siapa lagi yang menerima email kita.
func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
	to := sanitizeHeader(msg.To)
	if to == "" {
		return fmt.Errorf("alamat tujuan kosong atau tidak valid")
	}

	from := fmt.Sprintf("%s <%s>", sanitizeHeader(m.cfg.FromName), m.cfg.FromEmail)

	var body bytes.Buffer
	fmt.Fprintf(&body, "From: %s\r\n", from)
	fmt.Fprintf(&body, "To: %s\r\n", to)
	fmt.Fprintf(&body, "Subject: %s\r\n", sanitizeHeader(msg.Subject))
	body.WriteString("MIME-Version: 1.0\r\n")
	body.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	body.WriteString("\r\n")
	body.WriteString(msg.HTML)

	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)

	// net/smtp tidak menerima context, jadi pembatalan dijaga lewat channel.
	// Worker memakai ini untuk berhenti cepat saat shutdown.
	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(addr, auth, m.cfg.FromEmail, []string{to}, body.Bytes())
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("smtp send: %w", err)
		}
		return nil
	}
}

// sanitizeHeader membuang CR/LF untuk mencegah header injection.
func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return strings.TrimSpace(s)
}

// ============================================================================
// LogMailer — pengembangan lokal
// ============================================================================

// LogMailer hanya mencatat email ke log, tidak mengirim apa pun.
// Dipakai saat pengembangan supaya tidak perlu SMTP server.
type LogMailer struct {
	log *slog.Logger
}

func NewLogMailer(log *slog.Logger) *LogMailer {
	return &LogMailer{log: log}
}

func (m *LogMailer) Send(_ context.Context, msg Message) error {
	m.log.Info("email (tidak dikirim, mode log)",
		"to", msg.To,
		"subject", msg.Subject,
		"body_length", len(msg.HTML),
	)
	return nil
}

// ============================================================================
// Template invoice
// ============================================================================

// InvoiceData adalah isi struk yang dikirim ke pelanggan.
type InvoiceData struct {
	OrderNumber   string
	PaymentRef    string
	CustomerName  string
	TableNumber   string
	PaymentMethod string
	PaidAt        time.Time
	Items         []InvoiceItem
	Total         int64
	ShopName      string
}

type InvoiceItem struct {
	Name     string
	Quantity int
	Price    int64
	Subtotal int64
	Addons   []InvoiceAddon
	Notes    string
}

type InvoiceAddon struct {
	Name     string
	Quantity int
	Subtotal int64
}

// invoiceTemplate memakai html/template, bukan text/template — nama pelanggan
// dan catatan item berasal dari input pengguna, jadi harus di-escape.
var invoiceTemplate = template.Must(
	template.New("invoice").
		Funcs(template.FuncMap{"rupiah": formatRupiah}).
		Parse(`<!DOCTYPE html>
<html lang="id">
<head><meta charset="UTF-8"><title>Struk {{.OrderNumber}}</title></head>
<body style="margin:0;padding:24px;background:#f5f5f4;font-family:-apple-system,Segoe UI,Roboto,sans-serif;color:#1c1917">
<div style="max-width:560px;margin:0 auto;background:#fff;border-radius:12px;padding:32px">

  <h1 style="margin:0 0 4px;font-size:20px">{{.ShopName}}</h1>
  <p style="margin:0 0 24px;color:#78716c;font-size:14px">Terima kasih atas pesanan Anda</p>

  <table style="width:100%;font-size:14px;margin-bottom:24px">
    <tr><td style="color:#78716c;padding:2px 0">No. Pesanan</td><td style="text-align:right"><strong>{{.OrderNumber}}</strong></td></tr>
    <tr><td style="color:#78716c;padding:2px 0">Waktu</td><td style="text-align:right">{{.PaidAt.Format "02 Jan 2006, 15:04"}}</td></tr>
    {{if .TableNumber}}<tr><td style="color:#78716c;padding:2px 0">Meja</td><td style="text-align:right">{{.TableNumber}}</td></tr>{{end}}
    {{if .CustomerName}}<tr><td style="color:#78716c;padding:2px 0">Nama</td><td style="text-align:right">{{.CustomerName}}</td></tr>{{end}}
    <tr><td style="color:#78716c;padding:2px 0">Pembayaran</td><td style="text-align:right">{{.PaymentMethod}}</td></tr>
  </table>

  <table style="width:100%;font-size:14px;border-collapse:collapse">
    {{range .Items}}
    <tr>
      <td style="padding:10px 0;border-top:1px solid #e7e5e4">
        {{.Name}} <span style="color:#78716c">&times;{{.Quantity}}</span>
        {{if .Notes}}<div style="color:#a8a29e;font-size:12px;font-style:italic">{{.Notes}}</div>{{end}}
        {{range .Addons}}<div style="color:#78716c;font-size:12px;padding-left:12px">+ {{.Name}} &times;{{.Quantity}} &mdash; {{rupiah .Subtotal}}</div>{{end}}
      </td>
      <td style="padding:10px 0;border-top:1px solid #e7e5e4;text-align:right;white-space:nowrap">{{rupiah .Subtotal}}</td>
    </tr>
    {{end}}
    <tr>
      <td style="padding:14px 0;border-top:2px solid #1c1917"><strong>Total</strong></td>
      <td style="padding:14px 0;border-top:2px solid #1c1917;text-align:right"><strong>{{rupiah .Total}}</strong></td>
    </tr>
  </table>

  <p style="margin:24px 0 0;color:#a8a29e;font-size:12px">
    Ref: {{.PaymentRef}}<br>
    Email ini dikirim otomatis, mohon tidak dibalas.
  </p>

</div>
</body>
</html>`))

// RenderInvoice menyusun email struk.
func RenderInvoice(data InvoiceData) (Message, error) {
	var buf bytes.Buffer
	if err := invoiceTemplate.Execute(&buf, data); err != nil {
		return Message{}, fmt.Errorf("render invoice: %w", err)
	}

	return Message{
		Subject: fmt.Sprintf("Struk Pesanan %s — %s", data.OrderNumber, data.ShopName),
		HTML:    buf.String(),
	}, nil
}

// formatRupiah menulis angka dengan pemisah ribuan: 25000 → "Rp 25.000".
func formatRupiah(amount int64) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}

	digits := fmt.Sprintf("%d", amount)
	var out strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte('.')
		}
		out.WriteRune(d)
	}

	if negative {
		return "-Rp " + out.String()
	}
	return "Rp " + out.String()
}