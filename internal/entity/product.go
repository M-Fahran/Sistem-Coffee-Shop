package entity

type Category struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"not null"`
}

type Product struct {
	ID         uint   `gorm:"primaryKey"`
	CategoryID uint   `gorm:"not null"`
	Name       string `gorm:"not null"`
	BasePrice  int    `gorm:"not null"`
	Stock      int    `gorm:"not null"`
	IsActive   bool   `gorm:"default:true"`
}

type ProductAddon struct {
	ID       uint   `gorm:"primaryKey"`
	Name     string `gorm:"not null"`
	Price    int    `gorm:"not null"`
	Stock    int    `gorm:"not null"`
	IsActive bool   `gorm:"default:true"`
}
