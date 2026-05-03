package entity

type Product struct {
	ID         int64          `json:"id"`
	CategoryID int64          `json:"category_id"`
	Name       string         `json:"name"`
	BasePrice  int            `json:"base_price"`
	Stock      int            `json:"stock"`
	IsActive   bool           `json:"is_active"`
	AddOn      []ProductAddon `json:"addOn"`
}

type ProductAddon struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Price    int    `json:"price"`
	Stock    int    `json:"stock"`
	IsActive bool   `json:"is_active"`
}
