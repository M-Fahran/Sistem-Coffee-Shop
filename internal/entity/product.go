package entity

type Category struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"not null"`
}

type Product struct {
	ID         int    `json:"id"`
	CategoryID int    `json:"category_id"`
	Name       string `json:"name"`
	BasePrice  int    `json:"base_price"`
	Stock      int    `json:"stock"`
	IsActive   bool   `json:"is_active"`
}

type ProductAddon struct {
	ID       uint   `gorm:"primaryKey"`
	Name     string `gorm:"not null"`
	Price    int    `gorm:"not null"`
	Stock    int    `gorm:"not null"`
	IsActive bool   `gorm:"default:true"`
}
