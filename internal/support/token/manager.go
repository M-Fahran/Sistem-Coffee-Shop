// Package token menangani penerbitan dan verifikasi JWT.
//
// Ada dua jenis token yang tidak bisa saling ditukar:
//
//	staff_access  — untuk admin & kasir, hasil login username/password
//	table_session — untuk pelanggan, hasil menukar QR token meja
//
// Keduanya ditandatangani dengan kunci turunan yang berbeda (lihat deriveKey),
// jadi session token pelanggan secara kriptografis tidak mungkin dipakai
// sebagai token staff, bahkan kalau isi claim-nya dipalsukan.
//
// Package ini sengaja tidak menyentuh Redis maupun database — supaya bisa
// diuji tanpa infrastruktur. Pengecekan pencabutan dilakukan di middleware.
package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"coffeeshop/internal/config"
)

const (
	TypeStaffAccess  = "staff_access"
	TypeTableSession = "table_session"
)

const (
	// TableSessionTTL adalah umur session pelanggan setelah scan QR.
	TableSessionTTL = 2 * time.Hour

	clockSkew = 30 * time.Second
)

// signingMethod dikunci ke HS256 — menutup algorithm confusion.
var signingMethod = jwt.SigningMethodHS256

var (
	ErrInvalidToken = errors.New("token tidak valid")
	ErrWrongType    = errors.New("jenis token tidak sesuai")
)

// ============================================================================
// Scope pencabutan
// ============================================================================

// TableScope dan UserScope membentuk kunci scope yang dipakai epoch
// pencabutan. Keduanya ada di sini supaya service dan middleware tidak
// mungkin memakai format string yang berbeda.
func TableScope(tableID int64) string { return fmt.Sprintf("table:%d", tableID) }
func UserScope(userID int64) string   { return fmt.Sprintf("user:%d", userID) }

// ============================================================================
// Claims
// ============================================================================

// Claims adalah yang dibutuhkan middleware untuk mengecek pencabutan,
// tanpa peduli jenis tokennya.
type Claims interface {
	TokenID() string
	Scope() string
	EpochAt() int64
}

type StaffClaims struct {
	UserID int64  `json:"uid"`
	Role   string `json:"role"`
	Type   string `json:"typ"`
	Epoch  int64  `json:"epc"`
	jwt.RegisteredClaims
}

func (c *StaffClaims) TokenID() string { return c.ID }
func (c *StaffClaims) Scope() string   { return UserScope(c.UserID) }
func (c *StaffClaims) EpochAt() int64  { return c.Epoch }

type TableClaims struct {
	TableID     int64  `json:"tid"`
	TableNumber string `json:"tnum"`
	Type        string `json:"typ"`
	Epoch       int64  `json:"epc"`
	jwt.RegisteredClaims
}

func (c *TableClaims) TokenID() string { return c.ID }
func (c *TableClaims) Scope() string   { return TableScope(c.TableID) }
func (c *TableClaims) EpochAt() int64  { return c.Epoch }

// ============================================================================
// Manager
// ============================================================================

type Manager struct {
	staffKey  []byte
	tableKey  []byte
	issuer    string
	audience  string
	accessTTL time.Duration
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		staffKey:  deriveKey(cfg.JWT.Secret, TypeStaffAccess),
		tableKey:  deriveKey(cfg.JWT.Secret, TypeTableSession),
		issuer:    cfg.JWT.Issuer,
		audience:  cfg.JWT.Audience,
		accessTTL: cfg.JWT.AccessExpireDuration(),
	}
}

// deriveKey membuat kunci terpisah per jenis token dari satu JWT_SECRET,
// supaya pemisahan staff/pelanggan tidak cuma bergantung pada claim "typ".
func deriveKey(secret, purpose string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("coffeeshop/v1/" + purpose))
	return mac.Sum(nil)
}

// ============================================================================
// Issue
// ============================================================================

// IssueStaffToken menerbitkan access token untuk admin/kasir.
// epoch diambil pemanggil dari RevocationStore.CurrentEpoch(UserScope(id)).
func (m *Manager) IssueStaffToken(userID int64, role string, epoch int64) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.accessTTL)

	claims := StaffClaims{
		UserID:           userID,
		Role:             role,
		Type:             TypeStaffAccess,
		Epoch:            epoch,
		RegisteredClaims: m.registered(UserScope(userID), now, expiresAt),
	}
	signed, err := m.sign(claims, m.staffKey)
	return signed, expiresAt, err
}

// IssueTableSession menerbitkan session pelanggan untuk satu meja.
// epoch diambil pemanggil dari RevocationStore.CurrentEpoch(TableScope(id)).
func (m *Manager) IssueTableSession(tableID int64, tableNumber string, epoch int64) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(TableSessionTTL)

	claims := TableClaims{
		TableID:          tableID,
		TableNumber:      tableNumber,
		Type:             TypeTableSession,
		Epoch:            epoch,
		RegisteredClaims: m.registered(TableScope(tableID), now, expiresAt),
	}
	signed, err := m.sign(claims, m.tableKey)
	return signed, expiresAt, err
}

func (m *Manager) registered(subject string, now, expiresAt time.Time) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Subject:   subject,
		Issuer:    m.issuer,
		Audience:  jwt.ClaimStrings{m.audience},
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		ID:        newJTI(),
	}
}

func (m *Manager) sign(claims jwt.Claims, key []byte) (string, error) {
	signed, err := jwt.NewWithClaims(signingMethod, claims).SignedString(key)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// ============================================================================
// Parse
// ============================================================================

func (m *Manager) ParseStaffToken(raw string) (*StaffClaims, error) {
	claims := &StaffClaims{}
	if err := m.parse(raw, claims, m.staffKey); err != nil {
		return nil, err
	}
	if claims.Type != TypeStaffAccess {
		return nil, ErrWrongType
	}
	if claims.UserID <= 0 || claims.Role == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (m *Manager) ParseTableSession(raw string) (*TableClaims, error) {
	claims := &TableClaims{}
	if err := m.parse(raw, claims, m.tableKey); err != nil {
		return nil, err
	}
	if claims.Type != TypeTableSession {
		return nil, ErrWrongType
	}
	if claims.TableID <= 0 {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (m *Manager) parse(raw string, claims jwt.Claims, key []byte) error {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{signingMethod.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(clockSkew),
	)

	tok, err := parser.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return key, nil
	})
	if err != nil || !tok.Valid {
		return ErrInvalidToken
	}
	return nil
}

func newJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}