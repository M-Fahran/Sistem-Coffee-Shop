package service

import (
	"context"
	"errors"
	"os"
	"time"

	"coffeeshop/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	repo *repository.UserRepository
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Auth(ctx context.Context, req LoginRequest) (string, error) {
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return "", errors.New("email atau password salah")
	}

	if !user.IsActive {
		return 	"", errors.New("akun ini sedang dinonaktifkan")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		return "", errors.New("email atau password salah")
	}

	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		return "", errors.New("pengaturan server belum lengkap (JWT_SECRET hilang)")
	}

	claims := jwt.MapClaims{
		"user_id": user.ID,
		"role": user.Role,
		"exp": time.Now().Add(time.Hour * 1).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", errors.New("gagal proses login")
	}

	return signedToken, nil
}
