// Package worker menjalankan pekerjaan latar belakang.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"coffeeshop/internal/config"
	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/support/mailer"
)

const (
	// pollInterval adalah jeda saat antrean kosong. Cukup cepat supaya
	// invoice terasa langsung sampai, cukup lambat supaya database tidak
	// diketuk terus-menerus tanpa hasil.
	pollInterval = 3 * time.Second

	// batchSize membatasi jumlah job per putaran, supaya satu worker tidak
	// memonopoli antrean saat ada lonjakan.
	batchSize = 10

	// jobTimeout membatasi satu pekerjaan. SMTP yang menggantung tidak boleh
	// menahan seluruh antrean.
	jobTimeout = 30 * time.Second
)

type OutboxWorker struct {
	outbox *repository.OutboxRepository
	orders *repository.OrdersRepository
	tables *repository.TableRepository
	mail   mailer.Mailer
	log    *slog.Logger

	shopName string
}

func NewOutboxWorker(
	outbox *repository.OutboxRepository,
	orders *repository.OrdersRepository,
	tables *repository.TableRepository,
	mail mailer.Mailer,
	log *slog.Logger,
	shopName string,
) *OutboxWorker {
	return &OutboxWorker{
		outbox:   outbox,
		orders:   orders,
		tables:   tables,
		mail:     mail,
		log:      log,
		shopName: shopName,
	}
}

// Run memproses antrean sampai ctx dibatalkan.
//
// Dipanggil dari bootstrap sebagai goroutine, dan berhenti sendiri saat
// sinyal shutdown datang — job yang sedang jalan diselesaikan dulu, job
// berikutnya tidak diambil.
func (w *OutboxWorker) Run(ctx context.Context) {
	w.log.Info("outbox worker started", "poll_interval", pollInterval.String())

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("outbox worker stopped")
			return
		case <-ticker.C:
			w.drain(ctx)
		}
	}
}

// drain memproses job sampai antrean habis atau ctx dibatalkan.
//
// Perulangan di dalam satu tick penting: kalau ada 50 job menumpuk dan tiap
// tick hanya mengambil 10, antrean butuh 15 detik untuk habis. Dengan drain,
// selesai dalam satu putaran.
func (w *OutboxWorker) drain(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		jobs, err := w.outbox.ClaimJobs(ctx, batchSize)
		if err != nil {
			w.log.Error("claim outbox jobs failed", "err", err)
			return
		}
		if len(jobs) == 0 {
			return
		}

		for _, job := range jobs {
			w.process(ctx, job)
		}
	}
}

func (w *OutboxWorker) process(ctx context.Context, job repository.OutboxJob) {
	jobCtx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()

	err := w.execute(jobCtx, job)
	if err == nil {
		if markErr := w.outbox.MarkDone(context.WithoutCancel(ctx), job.ID); markErr != nil {
			// Job sudah terkirim tapi gagal ditandai selesai. Percobaan
			// berikutnya akan mengirim ulang — itulah sebabnya email invoice
			// harus aman dikirim dua kali (idempoten dari sisi pelanggan:
			// isinya sama persis).
			w.log.Error("mark job done failed", "job_id", job.ID, "err", markErr)
		}
		return
	}

	// context.WithoutCancel supaya kegagalan tetap tercatat meski shutdown
	// sedang berlangsung. Tanpa ini, job hilang dari radar.
	if markErr := w.outbox.MarkFailed(context.WithoutCancel(ctx), job, err); markErr != nil {
		w.log.Error("mark job failed failed", "job_id", job.ID, "err", markErr)
	}

	attempt := job.Attempts + 1
	level := slog.LevelWarn
	if attempt >= job.MaxAttempts {
		level = slog.LevelError // sudah menyerah, perlu perhatian manusia
	}
	w.log.Log(ctx, level, "outbox job failed",
		"job_id", job.ID,
		"job_type", job.JobType.String(),
		"attempt", attempt,
		"max_attempts", job.MaxAttempts,
		"err", err,
	)
}

func (w *OutboxWorker) execute(ctx context.Context, job repository.OutboxJob) error {
	switch job.JobType {
	case config.JobTypeInvoiceEmail:
		return w.sendInvoice(ctx, job.Payload)
	case config.JobTypeOrderReadyEmail:
		return w.sendOrderReady(ctx, job.Payload)
	default:
		// Job type tidak dikenal berarti ada versi kode lebih baru yang
		// menulisnya. Kembalikan error supaya job tidak hilang — nanti
		// terproses setelah deploy menyusul.
		return fmt.Errorf("job type tidak dikenal: %d", int16(job.JobType))
	}
}

// ============================================================================
// Invoice
// ============================================================================

func (w *OutboxWorker) sendInvoice(ctx context.Context, raw json.RawMessage) error {
	var payload repository.InvoiceJobPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode invoice payload: %w", err)
	}

	// Isi struk diambil dari database, bukan dari payload job. Payload hanya
	// membawa rujukan, jadi invoice selalu mencerminkan keadaan terkini.
	order, err := w.orders.GetOrderByID(ctx, payload.OrderID)
	if err != nil {
		return fmt.Errorf("ambil order %d: %w", payload.OrderID, err)
	}

	items, err := w.orders.GetItemsByOrderID(ctx, payload.OrderID)
	if err != nil {
		return fmt.Errorf("ambil item order %d: %w", payload.OrderID, err)
	}
	order.Items = items

	data := w.buildInvoiceData(ctx, order, payload)

	msg, err := mailer.RenderInvoice(data)
	if err != nil {
		return err
	}
	msg.To = payload.CustomerEmail

	if err := w.mail.Send(ctx, msg); err != nil {
		return fmt.Errorf("kirim invoice ke %s: %w", payload.CustomerEmail, err)
	}

	w.log.Info("invoice sent",
		"order_number", order.OrderNumber,
		"to", payload.CustomerEmail,
	)
	return nil
}

func (w *OutboxWorker) buildInvoiceData(
	ctx context.Context,
	order entity.Order,
	payload repository.InvoiceJobPayload,
) mailer.InvoiceData {
	data := mailer.InvoiceData{
		OrderNumber: order.OrderNumber,
		PaymentRef:  payload.PaymentRef,
		Total:       order.Subtotal,
		ShopName:    w.shopName,
		PaidAt:      order.UpdatedAt,
	}

	if order.CustomerName != nil {
		data.CustomerName = *order.CustomerName
	}

	// Nomor meja bersifat pelengkap: kegagalan mengambilnya tidak boleh
	// membatalkan pengiriman struk.
	if order.TableID != nil {
		if table, err := w.tables.FindByID(ctx, *order.TableID); err == nil {
			data.TableNumber = table.Number
		} else {
			w.log.Warn("ambil nomor meja gagal", "table_id", *order.TableID, "err", err)
		}
	}

	for _, item := range order.Items {
		invoiceItem := mailer.InvoiceItem{
			Name:     item.ProductName,
			Quantity: item.Quantity,
			Price:    item.UnitPrice,
			Subtotal: item.Subtotal,
		}
		if item.Notes != nil {
			invoiceItem.Notes = *item.Notes
		}
		for _, addon := range item.Addons {
			invoiceItem.Subtotal += addon.Subtotal
			invoiceItem.Addons = append(invoiceItem.Addons, mailer.InvoiceAddon{
				Name:     addon.AddonName,
				Quantity: addon.Quantity,
				Subtotal: addon.Subtotal,
			})
		}
		data.Items = append(data.Items, invoiceItem)
	}

	return data
}

// ============================================================================
// Pesanan siap
// ============================================================================

func (w *OutboxWorker) sendOrderReady(ctx context.Context, raw json.RawMessage) error {
	var payload repository.InvoiceJobPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode order ready payload: %w", err)
	}

	if payload.CustomerEmail == "" {
		return errors.New("email tujuan kosong")
	}

	msg := mailer.Message{
		To:      payload.CustomerEmail,
		Subject: fmt.Sprintf("Pesanan %s sudah siap — %s", payload.OrderNumber, w.shopName),
		HTML: fmt.Sprintf(
			`<p>Halo,</p><p>Pesanan <strong>%s</strong> sudah siap diambil. Terima kasih!</p>`,
			payload.OrderNumber),
	}

	if err := w.mail.Send(ctx, msg); err != nil {
		return fmt.Errorf("kirim notifikasi siap: %w", err)
	}
	return nil
}