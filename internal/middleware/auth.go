package middleware

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"

	"coffeeshop/internal/support/exception"
	"coffeeshop/internal/support/token"
)

// Kunci context. Selalu pakai konstanta ini, jangan string literal —
// typo pada c.Get("user_id") gagal diam-diam dan bikin auth bocor.
const (
	CtxUserID      = "user_id"
	CtxRole        = "role"
	CtxTableID     = "table_id"
	CtxTableNumber = "table_number"
	CtxTokenID     = "token_id"
	CtxExpiresAt   = "token_expires_at"
)

const bearerPrefix = "Bearer "

// Revoker adalah kontrak minimal yang dibutuhkan middleware untuk mengecek
// pencabutan. Dibuat interface (bukan *cache.RevocationStore langsung) supaya
// middleware bisa diuji tanpa Redis.
type Revoker interface {
	IsRevoked(ctx context.Context, jti, scope string, tokenEpoch int64) (bool, error)
}

// RequireStaff memvalidasi access token admin/kasir.
func RequireStaff(tm *token.Manager, rev Revoker) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := bearerToken(c)
		if !ok {
			abort(c, exception.Unauthorized("AUTH_401", "authentication required"))
			return
		}

		claims, err := tm.ParseStaffToken(raw)
		if err != nil {
			abort(c, exception.Unauthorized("AUTH_401", "invalid or expired token"))
			return
		}

		if revoked(c, rev, claims) {
			abort(c, exception.Unauthorized("AUTH_401", "session has been revoked, please log in again"))
			return
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxRole, claims.Role)
		c.Set(CtxTokenID, claims.TokenID())
		if claims.ExpiresAt != nil {
			c.Set(CtxExpiresAt, claims.ExpiresAt.Time)
		}
		c.Next()
	}
}

// RequireRole membatasi akses ke role tertentu. Harus dipasang setelah
// RequireStaff.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		role, ok := Role(c)
		if !ok {
			abort(c, exception.Unauthorized("AUTH_401", "authentication required"))
			return
		}
		if _, permitted := allowed[role]; !permitted {
			abort(c, exception.Forbidden("AUTH_403", "insufficient permission"))
			return
		}
		c.Next()
	}
}

// RequireTableSession memvalidasi session pelanggan hasil scan QR.
//
// Handler di belakangnya mengambil meja lewat middleware.TableID(c) —
// tidak boleh dari body request, karena itu artinya pelanggan bisa memesan
// atas nama meja lain.
func RequireTableSession(tm *token.Manager, rev Revoker) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := bearerToken(c)
		if !ok {
			abort(c, exception.Unauthorized("SESSION_401", "table session required"))
			return
		}

		claims, err := tm.ParseTableSession(raw)
		if err != nil {
			abort(c, exception.Unauthorized("SESSION_401", "session expired, please rescan the QR code"))
			return
		}

		if revoked(c, rev, claims) {
			abort(c, exception.Unauthorized("SESSION_401", "session ended, please rescan the QR code"))
			return
		}

		c.Set(CtxTableID, claims.TableID)
		c.Set(CtxTableNumber, claims.TableNumber)
		c.Set(CtxTokenID, claims.TokenID())
		if claims.ExpiresAt != nil {
			c.Set(CtxExpiresAt, claims.ExpiresAt.Time)
		}
		c.Next()
	}
}

// revoked mengecek denylist dan epoch dalam satu round trip Redis.
//
// Kalau Redis tidak bisa dihubungi, request DILANJUTKAN (fail-open) dan
// dicatat sebagai warning. Alasannya: Redis mati sebaiknya tidak
// mematikan seluruh kedai. Konsekuensinya, selama Redis down, session yang
// sudah dicabut masih bisa dipakai sampai token kedaluwarsa sendiri.
//
// Kalau kamu lebih memilih fail-closed, ganti `return false` di cabang
// error menjadi `return true`.
func revoked(c *gin.Context, rev Revoker, claims token.Claims) bool {
	if rev == nil {
		return false
	}

	isRevoked, err := rev.IsRevoked(
		c.Request.Context(),
		claims.TokenID(),
		claims.Scope(),
		claims.EpochAt(),
	)
	if err != nil {
		slog.Warn("revocation check failed, allowing request",
			"scope", claims.Scope(),
			"path", c.Request.URL.Path,
			"err", err,
		)
		return false
	}
	return isRevoked
}

// ============================================================================
// Accessor
// ============================================================================

func UserID(c *gin.Context) (int64, bool) {
	return get[int64](c, CtxUserID)
}

func Role(c *gin.Context) (string, bool) {
	return get[string](c, CtxRole)
}

func TableID(c *gin.Context) (int64, bool) {
	return get[int64](c, CtxTableID)
}

func TokenID(c *gin.Context) (string, bool) {
	return get[string](c, CtxTokenID)
}

func get[T any](c *gin.Context, key string) (T, bool) {
	var zero T
	v, ok := c.Get(key)
	if !ok {
		return zero, false
	}
	typed, ok := v.(T)
	if !ok {
		return zero, false
	}
	return typed, true
}

// ============================================================================
// Helper
// ============================================================================

// bearerToken membaca header Authorization dengan pengecekan skema yang ketat.
// "Bearer<token>" tanpa spasi ditolak, skema selain Bearer ditolak.
func bearerToken(c *gin.Context) (string, bool) {
	header := c.GetHeader("Authorization")
	if len(header) <= len(bearerPrefix) {
		return "", false
	}
	if !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return "", false
	}

	raw := strings.TrimSpace(header[len(bearerPrefix):])
	if raw == "" {
		return "", false
	}
	return raw, true
}

func abort(c *gin.Context, err error) {
	_ = c.Error(err)
	c.Abort()
}