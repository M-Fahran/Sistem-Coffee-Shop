package repository

import (
	"context"
	"coffeeshop/internal/entity"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductRepository interface {
	GetAllActiveProducts(ctx context.Context) ([]entity.Product, error)
}

type productRepository struct {
	db *pgxpool.Pool
}

func NewProductRepository(db *pgxpool.Pool) ProductRepository {
	return &productRepository{
		db: db,
	}
}

func (r *productRepository) GetAllActiveProducts(ctx context.Context) ([]entity.Product, error) {
	query := `
		SELECT id, category_id, name, base_price, stock, is_active 
		FROM products 
		WHERE is_active = true
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []entity.Product

	for rows.Next() {
		var p entity.Product
		
		err := rows.Scan(
			&p.ID,
			&p.CategoryID,
			&p.Name,
			&p.BasePrice,
			&p.Stock,
			&p.IsActive,
		)
		if err != nil {
			return nil, err
		}
		
		products = append(products, p)
	}

	return products, nil
}