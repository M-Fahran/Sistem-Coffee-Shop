package repository

import (
	"coffeeshop/internal/entity"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductRepository struct {
	db *pgxpool.Pool
}

func NewProductRepository(db *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) Create(ctx context.Context, p *entity.Product) error {
	query := `INSERT INTO products (category_id, name, base_price, stock, is_active)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`
	err := r.db.QueryRow(ctx, query, p.CategoryID, p.Name, p.BasePrice, p.Stock, p.IsActive).Scan(&p.ID)
	return err
}

func (r *ProductRepository) GetAllActive(ctx context.Context) ([]entity.Product, error) {
	query := `SELECT id, category_id, name, base_price, stock, is_active
			FROM products WHERE is_active = true`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var products []entity.Product
	for rows.Next() {
		var p entity.Product

		err := rows.Scan(&p.ID, &p.CategoryID, &p.Name, &p.BasePrice, &p.Stock, &p.IsActive)
		if err != nil {
			return nil, err
		}

		products = append(products, p)
	}

	if err := rows.Err();err != nil {
		return nil, err
	}
	return products, nil
}

func (r *ProductRepository) Update(ctx context.Context, p *entity.Product) error {
	query := `UPDATE products 
			SET category_id = $1, name = $2, base_price = $3, stock = $4, is_active = $5
			WHERE id = $6`
	
	commandTag, err := r.db.Exec(ctx, query, p.CategoryID, p.Name, p.BasePrice, p.Stock, p.IsActive, p.ID)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return context.DeadlineExceeded
	}

	return nil
}