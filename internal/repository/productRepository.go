package repository

import (
	"coffeeshop/internal/entity"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductRepository struct {
	db *pgxpool.Pool
}

type ProductAddOnRepository struct {
	db *pgxpool.Pool
}

func NewProductRepository(db *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{db: db}
}

func NewProductAddOnRepository(db *pgxpool.Pool) *ProductAddOnRepository {
	return &ProductAddOnRepository{db: db}
}

func (r *ProductRepository) CreateProduct(ctx context.Context, p *entity.Product) error {
	query := `INSERT INTO products (category_id, name, base_price, stock, is_active)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`
	err := r.db.QueryRow(ctx, query, p.CategoryID, p.Name, p.BasePrice, p.Stock, p.IsActive).Scan(&p.ID)
	return err
}

func (r *ProductRepository) GetAllProduct(ctx context.Context, filterStatus,categoryID string) ([]entity.Product, error) {
	query := `SELECT id, category_id, name, base_price, stock, is_active FROM products`

	var condition []string
	var args []interface{}
	argCounter := 1

	if categoryID != "" {
		condition = append(condition, fmt.Sprintf("category_id = $%d", argCounter))
		args = append(args, categoryID)
		argCounter++
	}

	if filterStatus == "true" {
		condition = append(condition, "is_active = true")
	} else if filterStatus == "false" {
		condition = append(condition, "is_active = false")
	}

	if len(condition) > 0 {
		query += " WHERE " + strings.Join(condition, " AND ")
	}
	query += ` ORDER BY id DESC`
	
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

func (r *ProductRepository) UpdateProduct(ctx context.Context, p *entity.Product) error {
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

func (r *ProductRepository) DeleteProduct(ctx context.Context, id int64) error {
	query := `DELETE FROM products WHERE ID = $1`
	
	commandTag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return errors.New("produk tidak ditemukan")
	}

	return nil
}

//Product AddOn

func (r *ProductAddOnRepository) CreateProductAddOn(ctx context.Context, p *entity.ProductAddon) error {
	query := `INSERT INTO product_addons (name, price, stock, is_active)
			VALUES ($1, $2, $3, $4) RETURNING id`
	err := r.db.QueryRow(ctx, query, p.Name, p.Price, p.Stock, p.IsActive).Scan(&p.ID)
	return err
}

func (r *ProductAddOnRepository) GetAllProductAddOn(ctx context.Context) ([]entity.ProductAddon, error) {
	query := `SELECT id, name, price, stock, is_active FROM product_addons`
	
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var productsAddon []entity.ProductAddon
	for rows.Next() {
		var p entity.ProductAddon

		err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock, &p.IsActive)
		if err != nil {
			return nil, err
		}

		productsAddon = append(productsAddon, p)
	}

	if err := rows.Err();err != nil {
		return nil, err
	}
	return productsAddon, nil
}

func (r *ProductAddOnRepository) UpdateProductAddOn(ctx context.Context, p *entity.ProductAddon) error {
	query := `UPDATE product_addons SET name = $1, price = $2, stock = $3, is_active = $4 WHERE id = $5`
	
	commandTag, err := r.db.Exec(ctx, query, p.Name, p.Price, p.Stock, p.IsActive, p.ID)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return context.DeadlineExceeded
	}

	return nil
}

func (r *ProductAddOnRepository) DeleteProductAddOn(ctx context.Context, id int64) error {
	query := `DELETE FROM product_addons WHERE ID = $1`
	
	commandTag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return errors.New("produk tidak ditemukan")
	}

	return nil
}