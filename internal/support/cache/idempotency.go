package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"coffeeshop/internal/config"
)

// Idempotency melindungi dari pesanan ganda.
//
// Kasus nyatanya: pelanggan menekan "Pesan", sinyal putus, dia menekan lagi.
// Tanpa penjaga ini, dua pesanan identik masuk dan dapurnya bikin dua kali.
//
// Alurnya:
//
//	Claim()   → menandai key sedang diproses (SETNX)
//	Commit()  → menyimpan order ID hasilnya
//	Release() → melepas klaim kalau pembuatan gagal, supaya bisa dicoba lagi
const (
	keyIdempotency = "idem:%s:%s" // scope, key

	// claimTTL cukup lama untuk menampung satu transaksi pembuatan pesanan,
	// tapi cukup pendek supaya request yang mati tidak mengunci key lama.
	claimTTL = 60 * time.Second

	// resultTTL adalah lama hasil disimpan. Retry setelah lewat ini akan
	// membuat pesanan baru — 24 jam jauh lebih panjang dari perilaku retry
	// klien mana pun.
	resultTTL = 24 * time.Hour

	claimMarker = "processing"
)

var (
	// ErrInProgress berarti request dengan key yang sama sedang diproses.
	ErrInProgress = errors.New("request dengan idempotency key ini sedang diproses")

	// ErrKeyUnclaimed berarti Commit dipanggil tanpa Claim yang berhasil.
	ErrKeyUnclaimed = errors.New("idempotency key belum diklaim")
)

type IdempotencyStore struct {
	rdb *config.RedisClient
}

func NewIdempotencyStore(rdb *config.RedisClient) *IdempotencyStore {
	return &IdempotencyStore{rdb: rdb}
}

// Claim mencoba menguasai satu idempotency key.
//
// Mengembalikan:
//
//	(0, false, nil)             → klaim berhasil, silakan lanjut membuat pesanan
//	(orderID, true, nil)        → key ini sudah pernah selesai, pakai hasil lama
//	(0, false, ErrInProgress)   → ada request kembar yang sedang berjalan
func (s *IdempotencyStore) Claim(ctx context.Context, scope, key string) (int64, bool, error) {
	redisKey := fmt.Sprintf(keyIdempotency, scope, key)

	claimed, err := s.rdb.SetNX(ctx, redisKey, claimMarker, claimTTL).Result()
	if err != nil {
		return 0, false, fmt.Errorf("idempotency claim: %w", err)
	}
	if claimed {
		return 0, false, nil
	}

	// Key sudah ada — entah masih diproses, atau sudah selesai.
	existing, err := s.rdb.Get(ctx, redisKey).Result()
	if err != nil {
		if errors.Is(err, config.RedisNil) {
			// Kedaluwarsa persis di antara SetNX dan Get. Perlakukan sebagai
			// bentrok; klien boleh mencoba lagi.
			return 0, false, ErrInProgress
		}
		return 0, false, fmt.Errorf("idempotency read: %w", err)
	}
	if existing == claimMarker {
		return 0, false, ErrInProgress
	}

	orderID, err := strconv.ParseInt(existing, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("idempotency: nilai rusak %q: %w", existing, err)
	}
	return orderID, true, nil
}

// Commit menyimpan hasil supaya retry berikutnya mengembalikan pesanan yang
// sama, bukan membuat yang baru.
func (s *IdempotencyStore) Commit(ctx context.Context, scope, key string, orderID int64) error {
	redisKey := fmt.Sprintf(keyIdempotency, scope, key)
	if err := s.rdb.Set(ctx, redisKey, orderID, resultTTL).Err(); err != nil {
		return fmt.Errorf("idempotency commit: %w", err)
	}
	return nil
}

// Release melepas klaim ketika pembuatan pesanan gagal, supaya klien bisa
// langsung mencoba lagi dengan key yang sama tanpa menunggu claimTTL habis.
func (s *IdempotencyStore) Release(ctx context.Context, scope, key string) error {
	redisKey := fmt.Sprintf(keyIdempotency, scope, key)
	if err := s.rdb.Del(ctx, redisKey).Err(); err != nil {
		return fmt.Errorf("idempotency release: %w", err)
	}
	return nil
}