package entity

import "time"

type User struct {
	ID        int64     `json:"id"`
	Email      string   `json:"email"`
	Username  string    `json:"username"`
	Password  string    `json:"-"`
	Role      string    `json:"role"`
	IsActive  bool    `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}
