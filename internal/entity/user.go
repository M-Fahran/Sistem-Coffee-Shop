package entity

import "time"

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Username  string    `json:"username"`
	Password  string    `json:"-"`
	Role      string    `json:"role"`
	IsActive  string    `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}
