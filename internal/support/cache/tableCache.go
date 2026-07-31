package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"coffeeshop/internal/config"
	"coffeeshop/internal/entity"
)

const (
	// tableTTL sengaja pendek. Ini batas atas berapa lama QR token yang
	// baru di-rotate masih bisa dipakai orang lain.
	tableTTL = 5 * time.Minute

	// negativeTTL untuk token yang terbukti tidak valid. Tanpa ini,
	// banjir token asal-asalan langsung mengenai Postgres.
	negativeTTL = 30 * time.Second

	negativeMarker = "\x00miss"
)

const (
	keyTableByID    = "table:id:%d"
	keyTableByToken = "table:tok:%s" // %s = SHA-256 dari QR token
)

// ErrNegativeHit berarti token ini sudah pernah dicek dan memang tidak ada.
// Caller boleh langsung menolak tanpa menyentuh database.
var ErrNegativeHit = errors.New("token diketahui tidak valid (negative cache)")

type TableCache struct {
	rdb *config.RedisClient
}

func NewTableCache(rdb *config.RedisClient) *TableCache {
	return &TableCache{rdb: rdb}
}

// ============================================================================
// Lookup by ID
// ============================================================================

// GetByID mengembalikan (nil, nil) kalau cache miss.
func (c *TableCache) GetByID(ctx context.Context, id int64) (*entity.Table, error) {
	return c.get(ctx, fmt.Sprintf(keyTableByID, id))
}

func (c *TableCache) SetByID(ctx context.Context, t *entity.Table) error {
	return c.set(ctx, fmt.Sprintf(keyTableByID, t.ID), t)
}

// ============================================================================
// Lookup by QR token
// ============================================================================

// GetByToken mengembalikan:
//
//	(table, nil)         → cache hit
//	(nil, ErrNegativeHit) → token sudah terbukti tidak valid
//	(nil, nil)           → cache miss, lanjut ke database
//
// QR token di-hash sebelum jadi key Redis supaya token asli tidak pernah
// tersimpan sebagai key — key bisa terbaca lewat MONITOR, SCAN, atau dump.
func (c *TableCache) GetByToken(ctx context.Context, qrToken string) (*entity.Table, error) {
	key := fmt.Sprintf(keyTableByToken, hashToken(qrToken))

	raw, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, config.RedisNil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cache get by token: %w", err)
	}
	if string(raw) == negativeMarker {
		return nil, ErrNegativeHit
	}

	var t entity.Table
	if err := json.Unmarshal(raw, &t); err != nil {
		_ = c.rdb.Del(ctx, key).Err()
		return nil, nil
	}
	return &t, nil
}

func (c *TableCache) SetByToken(ctx context.Context, t *entity.Table) error {
	return c.set(ctx, fmt.Sprintf(keyTableByToken, hashToken(t.QRToken)), t)
}

// SetTokenMiss menandai token sebagai tidak valid untuk sementara.
func (c *TableCache) SetTokenMiss(ctx context.Context, qrToken string) error {
	key := fmt.Sprintf(keyTableByToken, hashToken(qrToken))
	return c.rdb.Set(ctx, key, negativeMarker, negativeTTL).Err()
}

// ============================================================================
// Invalidasi
// ============================================================================

// Invalidate menghapus kedua key sekaligus. Wajib dipanggil setiap kali
// meja di-update, dihapus, atau QR token-nya di-rotate — kalau tidak,
// meja yang sudah dinonaktifkan masih bisa dipakai memesan sampai TTL habis.
func (c *TableCache) Invalidate(ctx context.Context, t *entity.Table) error {
	keys := []string{fmt.Sprintf(keyTableByID, t.ID)}
	if t.QRToken != "" {
		keys = append(keys, fmt.Sprintf(keyTableByToken, hashToken(t.QRToken)))
	}
	return c.rdb.Del(ctx, keys...).Err()
}

// ============================================================================
// Internal
// ============================================================================

func (c *TableCache) get(ctx context.Context, key string) (*entity.Table, error) {
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, config.RedisNil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cache get: %w", err)
	}

	var t entity.Table
	if err := json.Unmarshal(raw, &t); err != nil {
		_ = c.rdb.Del(ctx, key).Err()
		return nil, nil
	}
	return &t, nil
}

func (c *TableCache) set(ctx context.Context, key string, t *entity.Table) error {
	payload, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}
	return c.rdb.Set(ctx, key, payload, tableTTL).Err()
}

func hashToken(qrToken string) string {
	sum := sha256.Sum256([]byte(qrToken))
	return hex.EncodeToString(sum[:])
}