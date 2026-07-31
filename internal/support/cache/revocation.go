package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"coffeeshop/internal/config"
)

// Pencabutan token bekerja di dua tingkat:
//
//	denylist jti  — mencabut SATU token   (logout satu perangkat)
//	epoch scope   — mencabut SEMUA token  (kosongkan meja, rotate QR, pecat kasir)
//
// Epoch adalah angka yang naik satu setiap kali dicabut. Nilainya ikut
// ditanam di token saat diterbitkan; kalau epoch di Redis sudah lebih tinggi
// daripada yang ada di token, token itu dianggap kedaluwarsa.
const (
	keyRevokedJTI = "revoked:jti:%s"
	keyEpoch      = "session:epoch:%s"

	// epochTTL jauh lebih panjang daripada umur token terpanjang.
	// Kalau key ini kedaluwarsa lebih dulu, epoch balik ke 0 dan token
	// yang sudah dicabut hidup lagi.
	epochTTL = 30 * 24 * time.Hour

	// maxRevocationTTL adalah jaring pengaman kalau ada token dengan
	// masa berlaku tidak wajar.
	maxRevocationTTL = 24 * time.Hour
)

type RevocationStore struct {
	rdb *config.RedisClient
}

func NewRevocationStore(rdb *config.RedisClient) *RevocationStore {
	return &RevocationStore{rdb: rdb}
}

// ============================================================================
// Cabut satu token
// ============================================================================

// Revoke memasukkan satu token ke denylist sampai token itu kedaluwarsa
// dengan sendirinya. TTL-nya sengaja disamakan dengan sisa umur token —
// setelah lewat, tidak ada gunanya menyimpan entri ini.
func (s *RevocationStore) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	if jti == "" {
		return nil
	}

	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil // sudah kedaluwarsa, tidak perlu dicatat
	}
	if ttl > maxRevocationTTL {
		ttl = maxRevocationTTL
	}

	key := fmt.Sprintf(keyRevokedJTI, jti)
	if err := s.rdb.Set(ctx, key, "1", ttl).Err(); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	return nil
}

// ============================================================================
// Cabut semua token dalam satu scope
// ============================================================================

// CurrentEpoch membaca epoch berjalan. Scope belum pernah dicabut → 0.
func (s *RevocationStore) CurrentEpoch(ctx context.Context, scope string) (int64, error) {
	epoch, err := s.rdb.Get(ctx, fmt.Sprintf(keyEpoch, scope)).Int64()
	if errors.Is(err, config.RedisNil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read epoch %s: %w", scope, err)
	}
	return epoch, nil
}

// BumpEpoch mencabut seluruh token yang sudah terbit untuk scope ini.
// Dipakai saat meja dikosongkan, QR di-rotate, atau akun staff dinonaktifkan.
func (s *RevocationStore) BumpEpoch(ctx context.Context, scope string) (int64, error) {
	key := fmt.Sprintf(keyEpoch, scope)

	epoch, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("bump epoch %s: %w", scope, err)
	}
	if err := s.rdb.Expire(ctx, key, epochTTL).Err(); err != nil {
		return epoch, fmt.Errorf("refresh epoch ttl %s: %w", scope, err)
	}
	return epoch, nil
}

// ============================================================================
// Pengecekan (jalur panas)
// ============================================================================

// IsRevoked memeriksa denylist dan epoch sekaligus dalam SATU round trip
// Redis (~0.5 ms). Dipanggil di setiap request yang terautentikasi, jadi
// jangan dipecah jadi dua Get.
func (s *RevocationStore) IsRevoked(ctx context.Context, jti, scope string, tokenEpoch int64) (bool, error) {
	values, err := s.rdb.MGet(ctx,
		fmt.Sprintf(keyRevokedJTI, jti),
		fmt.Sprintf(keyEpoch, scope),
	).Result()
	if err != nil {
		return false, fmt.Errorf("revocation check: %w", err)
	}
	if len(values) != 2 {
		return false, fmt.Errorf("revocation check: unexpected response length %d", len(values))
	}

	// Token ini dicabut secara individual.
	if values[0] != nil {
		return true, nil
	}

	// Scope pernah dicabut massal setelah token ini terbit.
	if values[1] != nil {
		current, err := parseEpoch(values[1])
		if err != nil {
			return false, err
		}
		if tokenEpoch < current {
			return true, nil
		}
	}

	return false, nil
}

func parseEpoch(v any) (int64, error) {
	str, ok := v.(string)
	if !ok {
		return 0, fmt.Errorf("epoch: tipe tidak terduga %T", v)
	}
	epoch, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("epoch: nilai rusak %q: %w", str, err)
	}
	return epoch, nil
}