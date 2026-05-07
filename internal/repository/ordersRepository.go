package repository

import (
	"coffeeshop/internal/entity"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrdersRepository struct {
	db *pgxpool.Pool
}

func NewOrdersRepository(db *pgxpool.Pool) *OrdersRepository {
	return &OrdersRepository{db: db}
}

func (r *OrdersRepository) GetAllOrders(ctx context.Context) ([]entity.Orders, error) {
	query := `SELECT id, order_number, table_id, customer_name, payment_id, source, created_by_user_id, status, subtotal, created_at
			FROM orders`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []entity.Orders
	for rows.Next() {
		var order entity.Orders

		err := rows.Scan(&order.ID, &order.OrderNumber, &order.TableId, &order.CustomerName, &order.PaymentId, &order.Source, &order.CreatedByUserId, &order.Status, &order.SubTotal, &order.CreatedAt)
		if err != nil {
			return nil, err
		}

		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}

func (r *OrdersRepository) GetOrderByID(ctx context.Context, orderID int64) (entity.Orders, error){
	query := `SELECT id, order_number, table_id, customer_name, payment_id, source, created_by_user_id, status, subtotal, created_at FROM orders WHERE id = $1`

	var order entity.Orders
	err := r.db.QueryRow(ctx, query, orderID).Scan(
		&order.ID, &order.OrderNumber, &order.TableId, &order.CustomerName, &order.PaymentId, &order.Source, &order.CreatedByUserId, 
		&order.Status, &order.SubTotal, &order.CreatedAt,
	)
	if err != nil {
		return order, err
	}

	return order, nil
}

func (r *OrdersRepository) GetItemsByOrderID(ctx context.Context, orderID int64) ([]entity.OrderItems, error) {
	query := `SELECT id, order_id, product_id, product_name, unit_price, quantity, subtotal FROM order_items WHERE order_id = $1`

	rows, err := r.db.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []entity.OrderItems
	for rows.Next() {
		var item entity.OrderItems
		err := rows.Scan(&item.ID, &item.OrderId, &item.ProductId, &item.ProductName, &item.UnitPrice, &item.Quantity, &item.SubTotal)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}
