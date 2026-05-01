package entity

import "time"

type User struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Username  string    `gorm:"not null" json:"username"`
	Password  string    `gorm:"not null" json:"-"` 
	Role      string    `gorm:"type:varchar(50);not null;default:'cashier'" json:"role"`
	IsActive  bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
