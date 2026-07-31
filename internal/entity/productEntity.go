package entity

import "time"

// Product adalah menu yang dijual.
//
// BasePrice int64 dalam rupiah penuh — cocok dengan kolom BIGINT.
type Product struct {
	ID         int64          `json:"id"`
	CategoryID int64          `json:"category_id"`
	Name       string         `json:"name"`
	BasePrice  int64          `json:"base_price"`
	Stock      int            `json:"stock"`
	IsActive   bool           `json:"is_active"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Addons     []ProductAddon `json:"addons,omitempty"`
}

// ProductAddon adalah tambahan opsional pada produk.
type ProductAddon struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Stock    int    `json:"stock"`
	IsActive bool   `json:"is_active"`
}