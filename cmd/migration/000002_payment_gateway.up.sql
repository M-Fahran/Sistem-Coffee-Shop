-- ============================================================================
-- 000002 — payment gateway + outbox queue
--
-- Tiga hal yang ditambahkan:
--   1. Sequence untuk order_number & payment_ref
--   2. Kolom hasil gateway di payment_transactions
--   3. Tabel outbox_jobs (antrean invoice) dan webhook_events (audit + dedup)
-- ============================================================================

-- ----------------------------------------------------------------------------
-- 1. Sequence
--
-- Alternatifnya, SELECT COUNT(*)+1, punya race condition: dua pesanan yang
-- masuk bersamaan bisa mendapat nomor sama dan salah satunya gagal karena
-- UNIQUE constraint.
-- ----------------------------------------------------------------------------
CREATE SEQUENCE IF NOT EXISTS order_number_seq START 1;
CREATE SEQUENCE IF NOT EXISTS payment_ref_seq  START 1;

-- ----------------------------------------------------------------------------
-- 2. Kolom hasil gateway
-- ----------------------------------------------------------------------------

-- payment_code menampung QR string (QRIS), nomor VA, atau deeplink e-wallet.
-- Isinya panjang untuk QRIS, jadi text bukan varchar.
ALTER TABLE payment_transactions
  ADD COLUMN IF NOT EXISTS payment_code text;

-- expires_at adalah batas waktu pembayaran dari gateway. Dipakai job
-- kedaluwarsa untuk membatalkan pesanan yang tidak kunjung dibayar.
ALTER TABLE payment_transactions
  ADD COLUMN IF NOT EXISTS expires_at timestamptz;

-- Pencarian saat webhook masuk selalu lewat payment_ref (yang kita kirim ke
-- gateway sebagai order_id) atau external_id (ID milik gateway).
CREATE INDEX IF NOT EXISTS idx_payments_ref ON payment_transactions (payment_ref);

-- ----------------------------------------------------------------------------
-- 3a. outbox_jobs — antrean pekerjaan latar belakang
--
-- Kenapa tabel, bukan Redis:
--
-- Job invoice ditulis DI DALAM transaksi yang sama dengan perubahan status
-- pembayaran. Kalau ditulis ke Redis setelah commit, lalu proses mati di
-- antara keduanya, invoice hilang tanpa jejak. Dengan outbox, dua-duanya
-- commit bersama atau dua-duanya batal.
--
-- job_type: 1=invoice_email, 2=order_ready_email
-- status:   1=pending, 2=processing, 3=done, 4=failed
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS outbox_jobs (
  id           BIGSERIAL    PRIMARY KEY,
  job_type     smallint     NOT NULL CHECK (job_type IN (1, 2)),
  payload      jsonb        NOT NULL,
  status       smallint     NOT NULL DEFAULT 1 CHECK (status IN (1, 2, 3, 4)),
  attempts     int          NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  max_attempts int          NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
  last_error   text,

  -- dedup_key mencegah job kembar. Webhook gateway bisa datang berkali-kali
  -- untuk pembayaran yang sama; tanpa ini pelanggan menerima invoice ganda.
  dedup_key    varchar(160) UNIQUE,

  -- run_after menunda percobaan berikutnya (exponential backoff).
  run_after    timestamptz  NOT NULL DEFAULT now(),
  created_at   timestamptz  NOT NULL DEFAULT now(),
  updated_at   timestamptz  NOT NULL DEFAULT now()
);

-- Worker hanya peduli job yang siap dijalankan. Partial index membuat
-- tabelnya tetap ringan meski riwayat job menumpuk.
CREATE INDEX IF NOT EXISTS idx_outbox_ready
  ON outbox_jobs (run_after, id)
  WHERE status IN (1, 2);

-- ----------------------------------------------------------------------------
-- 3b. webhook_events — catatan mentah setiap notifikasi gateway
--
-- Gunanya dua: bukti saat ada sengketa pembayaran, dan bahan replay kalau
-- pemrosesan sempat bermasalah. Body disimpan apa adanya, tidak diolah.
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS webhook_events (
  id           BIGSERIAL    PRIMARY KEY,
  provider     varchar(50)  NOT NULL,

  -- event_key membuat notifikasi kembar tertolak di tingkat database.
  -- Isinya gabungan referensi transaksi + status, jadi notifikasi susulan
  -- dengan status berbeda tetap masuk.
  event_key    varchar(200) NOT NULL UNIQUE,

  payment_ref  varchar(100),
  raw_body     text         NOT NULL,
  received_at  timestamptz  NOT NULL DEFAULT now(),
  processed_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_webhook_events_ref
  ON webhook_events (payment_ref, received_at DESC);