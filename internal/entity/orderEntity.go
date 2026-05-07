package entity

import "time"

type Orders struct {
	ID              int64     `json:"id"`
	OrderNumber     string    `json:"order_number"`
	TableId         int64     `json:"table_id"`
	CustomerName    string    `json:"customer_name"`
	PaymentId       int64     `json:"payment_id"`
	Source          string    `json:"source"`
	CreatedByUserId int64     `json:"created_by_user_id"`
	Status          string    `json:"status"`
	SubTotal        float64   `json:"subtotal"`
	CreatedAt       time.Time `json:"created_at"`
	Items []OrderItems `json:"items,omitempty"`
}

type OrderItems struct {
	ID          int64   `json:"id"`
	OrderId     int64   `json:"order_id"`
	ProductId   int64   `json:"product_id"`
	ProductName string  `json:"product_name"`
	UnitPrice   float64 `json:"unit_price"`
	Quantity    int     `json:"quantity"`
	SubTotal    float64 `json:"subtotalS"`
}
