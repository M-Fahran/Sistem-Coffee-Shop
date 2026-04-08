package entity

type Category struct {
	ID   int64 `json:"id"`
	Name int64 `json:"name"`
}

type Product struct {
	ID         int64   `json:"id"`
	CategoryID int64   `json:"category_id"`
	Name       string  `json:"name"`
	BasePrice  float64 `json:"base_price"`
	Stock      int     `json:"stock"`
	IsActive   bool    `json:"is_active"`
}

type ProductAddon struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Stock    int     `json:"stock"`
	IsActive bool    `json:"is_active"`
}
