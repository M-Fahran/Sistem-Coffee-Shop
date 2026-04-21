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
	query := `INSERT INTO products (categort_id, name, base_price, stock, is_active)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`
	err := r.db.QueryRow(ctx, query, p.CategoryID, p.Name, p.BasePrice, p.Stock, p.IsActive).Scan(&p.ID)
	return err
}