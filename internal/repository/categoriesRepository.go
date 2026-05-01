package repository

import (
	"coffeeshop/internal/entity"
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoriesRepository struct {
	db *pgxpool.Pool
}

func NewCategoriesRepository(db *pgxpool.Pool) *CategoriesRepository {
	return &CategoriesRepository{db: db}
}

func (r *CategoriesRepository) GetAllCategories(ctx context.Context) ([]entity.Categories, error) {
	query := `SELECT id, name FROM categories`
	
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var categories []entity.Categories
	for rows.Next() {
		var p entity.Categories

		err := rows.Scan(&p.ID, &p.Name)
		if err != nil {
			return nil, err
		}

		categories = append(categories, p)
	}

	if err := rows.Err();err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *CategoriesRepository) Create(ctx context.Context, p *entity.Categories) error {
	query := `INSERT INTO categories (name) VALUES ($1) RETURNING id`
	err := r.db.QueryRow(ctx, query, p.Name).Scan(&p.ID)
	return err
}

func (r *CategoriesRepository) Update(ctx context.Context, p *entity.Categories) error {
	query := `UPDATE categories SET name = $1 WHERE id = $2`
	
	commandTag, err := r.db.Exec(ctx, query, p.Name , p.ID)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return context.DeadlineExceeded
	}

	return nil
}

func (r *CategoriesRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM categories WHERE ID = $1`
	
	commandTag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return errors.New("produk tidak ditemukan")
	}

	return nil
}