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
	query := `SELECT id, order_number, table_id, customer_name, payment_id, source, created_by_user_id, status, subtotal
			FROM orders`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var products []entity.Orders
	for rows.Next() {
		var p entity.Orders

		err := rows.Scan(&p.ID, &p.OrderNumber, &p.TableId, &p.CustomerName, &p.PaymentId, &p.Source, &p.CreatedByUserId, &p.Status, &p.SubTotal)
		if err != nil {
			return nil, err
		}

		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return products, nil
}