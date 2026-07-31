package entity

import (
	"time"

	"coffeeshop/internal/config"
)

// User adalah akun staff (admin atau kasir).
//
// ID int64 supaya seragam dengan entity lain — sebelumnya `uint`, yang
// memaksa konversi manual setiap kali dipakai.
//
// Tag GORM sudah dibuang: project ini memakai pgx, jadi tag tersebut
// metadata mati yang menyesatkan pembaca berikutnya.
type User struct {
	ID        int64           `json:"id"`
	Email     string          `json:"email"`
	Username  string          `json:"username"`
	Password  string          `json:"-"` // tidak pernah keluar lewat JSON
	Role      config.UserRole `json:"role"`
	IsActive  bool            `json:"is_active"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// IsAdmin lebih aman daripada membandingkan string.
func (u *User) IsAdmin() bool { return u.Role == config.UserRoleAdmin }