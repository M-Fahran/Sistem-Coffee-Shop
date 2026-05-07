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

func (r *ProductRepository) CreateProduct(ctx context.Context, p *entity.Product) (int64, error) {
	query := `INSERT INTO products (category_id, name, base_price, stock, is_active)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`
	
	var newID int64
	err := r.db.QueryRow(ctx, query, p.CategoryID, p.Name, p.BasePrice, p.Stock, p.IsActive).Scan(&newID)
	if err != nil {
		return 0, err
	}

	return newID, nil
}

func (r *ProductRepository) GetAllProduct(ctx context.Context, filterStatus, categoryID string) ([]entity.Product, error) {
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

func (r *ProductRepository) GetProductByID(ctx context.Context, id int64) (entity.Product, error){
	query := `SELECT id, category_id, name, base_price, stock, is_active FROM products WHERE id = $1`

	var p entity.Product
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.CategoryID, &p.Name, &p.BasePrice, &p.Stock, &p.IsActive,
	)

	if err != nil {
		return p, err
	}

	return p, nil
}

func (r *ProductRepository) GetAddOnByProductID(ctx context.Context, productID int64) ([]entity.ProductAddon, error){
	query := `SELECT pa.id, pa.name, pa.price, pa.stock, pa.is_active FROM product_addons pa JOIN product_addon_map pam ON pa.id = pam.product_addon_id WHERE pam.product_id = $1`

	rows, err := r.db.Query(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	addon := []entity.ProductAddon{}

	for rows.Next() {
		var a entity.ProductAddon
		err := rows.Scan(&a.ID, &a.Name, &a.Price, &a.Stock, &a.IsActive)
		if err != nil {
			return nil, err
		}
		addon = append(addon, a)
	}

	return addon, nil
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

func (r *ProductAddOnRepository) GetProductAddOnByID(ctx context.Context, id int64) (entity.ProductAddon, error){
	query := `SELECT id, name, price, stock, is_active FROM product_addons WHERE id = $1`

	var p entity.ProductAddon

	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.Price, &p.Stock, &p.IsActive,
	)

	if err != nil {
		return p, nil
	}

	return p, nil
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

func (r *ProductRepository) SyncProductAddOn(ctx context.Context, productID int64, addonID []int64) error {
	queryDelete := `DELETE FROM product_addon_map WHERE product_id = $1`
	_, err := r.db.Exec(ctx, queryDelete, productID)
	if err != nil {
		return err
	}

	if len(addonID) == 0 {
		return nil
	}

	queryInsert := `INSERT INTO product_addon_map (product_id, product_addon_id) VALUES ($1, $2)`
	for _, addonID := range addonID {
		_, err := r.db.Exec(ctx, queryInsert, productID, addonID)
		if err != nil {
			return err
		}
	}

	return nil
}