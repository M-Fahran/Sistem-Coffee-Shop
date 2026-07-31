package controller

import (
	"time"

	"github.com/gin-gonic/gin"

	"coffeeshop/internal/middleware"
	"coffeeshop/internal/request"
	"coffeeshop/internal/service"
	"coffeeshop/internal/support/exception"
	"coffeeshop/internal/support/response"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// loginResponse mengunci bentuk respons login.
//
// Sebelumnya pakai gin.H (map[string]any), jadi typo "toke" alih-alih
// "token" baru ketahuan di frontend. Dengan struct, compiler yang menangkap.
type loginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      userView  `json:"user"`
}

type userView struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// Login POST /api/v1/auth/login
func (h *UserHandler) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(exception.Validation(err))
		return
	}

	result, err := h.userService.Auth(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response.OK(c, "login successful", loginResponse{
		Token:     result.Token,
		ExpiresAt: result.ExpiresAt,
		User: userView{
			ID:       result.User.ID,
			Email:    result.User.Email,
			Username: result.User.Username,
			// Role disajikan sebagai string ("admin"/"cashier"), bukan angka —
			// kontrak API tetap terbaca meski di database SMALLINT.
			Role: result.User.Role.String(),
		},
	})
}

// Logout POST /api/v1/auth/logout
//
// Mencabut token yang sedang dipakai saja. jti dan waktu kedaluwarsa diambil
// dari context yang diisi middleware.RequireStaff — bukan dari body request,
// supaya tidak ada yang bisa mencabut token milik orang lain.
func (h *UserHandler) Logout(c *gin.Context) {
	jti, ok := middleware.TokenID(c)
	if !ok {
		_ = c.Error(exception.Unauthorized("AUTH_401", "authentication required"))
		return
	}

	var expiresAt time.Time
	if v, exists := c.Get(middleware.CtxExpiresAt); exists {
		if t, isTime := v.(time.Time); isTime {
			expiresAt = t
		}
	}

	if err := h.userService.Logout(c.Request.Context(), jti, expiresAt); err != nil {
		_ = c.Error(err)
		return
	}

	response.OK(c, "logged out successfully", nil)
}