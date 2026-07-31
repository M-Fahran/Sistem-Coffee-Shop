package service

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"coffeeshop/internal/config"
	"coffeeshop/internal/entity"
	"coffeeshop/internal/repository"
	"coffeeshop/internal/request"
	"coffeeshop/internal/support/cache"
	"coffeeshop/internal/support/exception"
	"coffeeshop/internal/support/token"
)

// dummyHash dipakai saat email tidak ditemukan, supaya waktu respons login
// untuk "email salah" dan "password salah" sama-sama menanggung biaya bcrypt.
// Tanpa ini, selisih waktu respons (~60 ms) membocorkan email mana yang
// terdaftar di sistem.
var dummyHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")

type UserService struct {
	repo *repository.UserRepository
	tm   *token.Manager
	rev  *cache.RevocationStore
}

func NewUserService(
	repo *repository.UserRepository,
	tm *token.Manager,
	rev *cache.RevocationStore,
) *UserService {
	return &UserService{repo: repo, tm: tm, rev: rev}
}

// AuthResult adalah hasil login yang siap dipakai controller.
type AuthResult struct {
	User      *entity.User
	Token     string
	ExpiresAt time.Time
}

// Auth memvalidasi kredensial dan menerbitkan access token.
//
// Semua kegagalan mengembalikan pesan yang sama ("invalid credentials").
// Membedakan "email tidak terdaftar", "password salah", dan "akun nonaktif"
// memudahkan penyerang memetakan akun yang ada.
func (s *UserService) Auth(ctx context.Context, req request.LoginRequest) (*AuthResult, error) {
	invalid := exception.Unauthorized("AUTH_401", "invalid credentials")

	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, config.ErrNoRows) {
			// Tetap jalankan bcrypt agar waktu respons konsisten.
			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password))
			return nil, invalid
		}
		return nil, exception.Internal(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, invalid
	}
	if !user.IsActive {
		return nil, invalid
	}

	// Epoch pencabutan ditanam di token. Kalau nanti seluruh session user ini
	// dicabut, epoch di Redis naik dan token lama otomatis ditolak.
	epoch, err := s.rev.CurrentEpoch(ctx, token.UserScope(user.ID))
	if err != nil {
		// Redis bermasalah bukan alasan menolak login. Token tetap terbit
		// dengan epoch 0; konsekuensinya pencabutan massal sebelumnya tidak
		// terbawa sampai Redis pulih.
		epoch = 0
	}

	signed, expiresAt, err := s.tm.IssueStaffToken(user.ID, user.Role.String(), epoch)
	if err != nil {
		return nil, exception.Internal(err)
	}

	return &AuthResult{User: user, Token: signed, ExpiresAt: expiresAt}, nil
}

// Logout mencabut satu token saja — perangkat lain tetap login.
func (s *UserService) Logout(ctx context.Context, jti string, expiresAt time.Time) error {
	if err := s.rev.Revoke(ctx, jti, expiresAt); err != nil {
		return exception.Internal(err)
	}
	return nil
}

// RevokeAllSessions mematikan seluruh token milik satu user sekaligus.
// Dipakai kalau akun dicurigai bocor atau staff berhenti bekerja.
func (s *UserService) RevokeAllSessions(ctx context.Context, userID int64) error {
	if _, err := s.rev.BumpEpoch(ctx, token.UserScope(userID)); err != nil {
		return exception.Internal(err)
	}
	return nil
}