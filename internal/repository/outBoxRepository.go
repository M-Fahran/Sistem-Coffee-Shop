package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"coffeeshop/internal/config"
	"coffeeshop/internal/support/exception"
)

// Parameter percobaan ulang.
//
// Backoff eksponensial: 1 menit, 2, 4, 8, 16. Kalau SMTP sedang bermasalah,
// percobaan yang menumpuk justru memperparah — jeda yang melebar memberi
// waktu pulih tanpa membuang job.
const (
	outboxBaseBackoff = 1 * time.Minute
	outboxMaxBackoff  = 30 * time.Minute

	// outboxStaleAfter mengembalikan job yang tersangkut di status
	// "processing" karena worker mati mendadak. Tanpa ini, job itu tidak
	// akan pernah diambil siapa pun lagi.
	outboxStaleAfter = 5 * time.Minute
)

type OutboxRepository struct {
	db *config.Pool
}

func NewOutboxRepository(db *config.Pool) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// OutboxJob adalah satu pekerjaan yang siap dijalankan.
type OutboxJob struct {
	ID          int64
	JobType     config.JobType
	Payload     json.RawMessage
	Attempts    int
	MaxAttempts int
}

// ClaimJobs mengambil sejumlah job dan menandainya sedang diproses.
//
// FOR UPDATE SKIP LOCKED adalah kunci pola ini: kalau nanti ada dua instance
// server berjalan bersamaan, worker kedua akan MELEWATI baris yang sudah
// dikunci worker pertama alih-alih menunggunya. Tanpa SKIP LOCKED, worker
// kedua memblokir dan antrean berjalan serial.
//
// Query-nya sekaligus mengubah status jadi "processing", jadi tidak ada celah
// antara membaca dan menandai.
func (r *OutboxRepository) ClaimJobs(ctx context.Context, limit int) ([]OutboxJob, error) {
	const query = `
		WITH ready AS (
			SELECT id
			FROM outbox_jobs
			WHERE run_after <= now()
			  AND (
			        status = $1
			     OR (status = $2 AND updated_at < now() - $3::interval)
			  )
			ORDER BY run_after, id
			LIMIT $4
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox_jobs o
		SET status = $2, updated_at = now()
		FROM ready
		WHERE o.id = ready.id
		RETURNING o.id, o.job_type, o.payload, o.attempts, o.max_attempts`

	rows, err := r.db.Query(ctx, query,
		config.JobStatusPending,
		config.JobStatusProcessing,
		outboxStaleAfter.String(),
		limit,
	)
	if err != nil {
		return nil, exception.Internal(fmt.Errorf("claim outbox jobs: %w", err))
	}
	defer rows.Close()

	jobs := make([]OutboxJob, 0, limit)
	for rows.Next() {
		var j OutboxJob
		if err := rows.Scan(&j.ID, &j.JobType, &j.Payload, &j.Attempts, &j.MaxAttempts); err != nil {
			return nil, exception.Internal(fmt.Errorf("claim outbox scan: %w", err))
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, exception.Internal(fmt.Errorf("claim outbox rows: %w", err))
	}
	return jobs, nil
}

// MarkDone menutup job yang berhasil.
func (r *OutboxRepository) MarkDone(ctx context.Context, jobID int64) error {
	const query = `
		UPDATE outbox_jobs
		SET status = $1, attempts = attempts + 1, last_error = NULL, updated_at = now()
		WHERE id = $2`

	if _, err := r.db.Exec(ctx, query, config.JobStatusDone, jobID); err != nil {
		return exception.Internal(fmt.Errorf("mark job done: %w", err))
	}
	return nil
}

// MarkFailed mencatat kegagalan dan menjadwalkan percobaan berikutnya.
//
// Setelah max_attempts habis, job berhenti di status "failed" — tidak dihapus,
// supaya bisa diperiksa manual. Invoice yang gagal terkirim adalah masalah
// yang perlu diketahui orang, bukan disembunyikan.
func (r *OutboxRepository) MarkFailed(ctx context.Context, job OutboxJob, cause error) error {
	nextAttempt := job.Attempts + 1
	exhausted := nextAttempt >= job.MaxAttempts

	status := config.JobStatusPending
	if exhausted {
		status = config.JobStatusFailed
	}

	const query = `
		UPDATE outbox_jobs
		SET status = $1, attempts = $2, last_error = $3,
		    run_after = now() + $4::interval, updated_at = now()
		WHERE id = $5`

	_, err := r.db.Exec(ctx, query,
		status,
		nextAttempt,
		truncateError(cause),
		backoffFor(nextAttempt).String(),
		job.ID,
	)
	if err != nil {
		return exception.Internal(fmt.Errorf("mark job failed: %w", err))
	}
	return nil
}

// CountFailed dipakai health check untuk memantau job yang menyerah.
func (r *OutboxRepository) CountFailed(ctx context.Context) (int, error) {
	const query = `SELECT count(*) FROM outbox_jobs WHERE status = $1`

	var total int
	if err := r.db.QueryRow(ctx, query, config.JobStatusFailed).Scan(&total); err != nil {
		return 0, exception.Internal(fmt.Errorf("count failed jobs: %w", err))
	}
	return total, nil
}

// backoffFor menghitung jeda percobaan berikutnya: 1, 2, 4, 8, 16 menit,
// dibatasi outboxMaxBackoff.
func backoffFor(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	backoff := time.Duration(math.Pow(2, float64(attempt-1))) * outboxBaseBackoff
	if backoff > outboxMaxBackoff || backoff <= 0 {
		return outboxMaxBackoff
	}
	return backoff
}

// truncateError menjaga kolom last_error tidak diisi stack trace raksasa.
func truncateError(err error) string {
	if err == nil {
		return ""
	}
	const max = 1000
	msg := err.Error()
	if len(msg) > max {
		return msg[:max]
	}
	return msg
}