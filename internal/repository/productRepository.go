package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"coffeeshop/internal/config"
	"coffeeshop/internal/entity"
)

type ProductRepository struct {
	db *config.Pool
}

type ProductAddOnRepository struct {
	db *config.Pool
}

func NewProductRepository(db *config.Pool) *ProductRepository {
	return &ProductRepository{db: db}
}

func NewProductAddOnRepository(db *config.Pool) *ProductAddOnRepository {
	return &ProductAddOnRepository{db: db}
}

var errProductNotFound = errors.New("produk tidak ditemukan")

// ============================================================================
// Product
// ============================================================================

func (r *ProductRepository) CreateProduct(ctx context.Context, p *entity.Product) (int64, error) {
	const query = `
		INSERT INTO products (category_id, name, base_price, stock, is_active)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`

	var newID int64
	err := r.db.QueryRow(ctx, query, p.CategoryID, p.Name, p.BasePrice, p.Stock, p.IsActive).Scan(&newID)
	if err != nil {
		return 0, err
	}
	return newID, nil
}

// GetAllProduct mengambil produk dengan filter opsional.
//
// categoryID datang sebagai string dari query param, tapi kolomnya BIGINT.
// Karena itu di-parse di sini — kalau dikirim mentah, pgx menolak encode
// string ke bigint dan errornya membingungkan.
func (r *ProductRepository) GetAllProduct(ctx context.Context, filterStatus, categoryID string) ([]entity.Product, error) {
	query := `
		SELECT id, category_id, name, base_price, stock, is_active, created_at, updated_at
		FROM products`

	var (
		conditions []string
		args       []any
		argIdx     = 1
	)

	if categoryID != "" {
		parsed, err := strconv.ParseInt(categoryID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("category_id harus berupa angka: %q", categoryID)
		}
		conditions = append(conditions, fmt.Sprintf("category_id = $%d", argIdx))
		args = append(args, parsed)
		argIdx++
	}

	switch filterStatus {
	case "true":
		conditions = append(conditions, "is_active = TRUE")
	case "false":
		conditions = append(conditions, "is_active = FALSE")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY id DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]entity.Product, 0, 32)
	for rows.Next() {
		var p entity.Product
		if err := rows.Scan(
			&p.ID, &p.CategoryID, &p.Name, &p.BasePrice,
			&p.Stock, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func (r *ProductRepository) GetProductByID(ctx context.Context, id int64) (entity.Product, error) {
	const query = `
		SELECT id, category_id, name, base_price, stock, is_active, created_at, updated_at
		FROM products WHERE id = $1`

	var p entity.Product
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.CategoryID, &p.Name, &p.BasePrice,
		&p.Stock, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, config.ErrNoRows) {
			return p, errProductNotFound
		}
		return p, err
	}
	return p, nil
}

func (r *ProductRepository) GetAddOnByProductID(ctx context.Context, productID int64) ([]entity.ProductAddon, error) {
	const query = `
		SELECT pa.id, pa.name, pa.price, pa.stock, pa.is_active
		FROM product_addons pa
		JOIN product_addon_map pam ON pa.id = pam.product_addon_id
		WHERE pam.product_id = $1
		ORDER BY pa.id`

	rows, err := r.db.Query(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	addons := make([]entity.ProductAddon, 0, 4)
	for rows.Next() {
		var a entity.ProductAddon
		if err := rows.Scan(&a.ID, &a.Name, &a.Price, &a.Stock, &a.IsActive); err != nil {
			return nil, err
		}
		addons = append(addons, a)
	}
	return addons, rows.Err()
}

func (r *ProductRepository) UpdateProduct(ctx context.Context, p *entity.Product) error {
	const query = `
		UPDATE products
		SET category_id = $1, name = $2, base_price = $3, stock = $4,
		    is_active = $5, updated_at = now()
		WHERE id = $6`

	tag, err := r.db.Exec(ctx, query, p.CategoryID, p.Name, p.BasePrice, p.Stock, p.IsActive, p.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Sebelumnya mengembalikan context.DeadlineExceeded di sini —
		// menyesatkan, karena tidak ada hubungannya dengan timeout.
		return errProductNotFound
	}
	return nil
}

func (r *ProductRepository) DeleteProduct(ctx context.Context, id int64) error {
	const query = `DELETE FROM products WHERE id = $1`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errProductNotFound
	}
	return nil
}

// SyncProductAddOn mengganti seluruh pemetaan add-on sebuah produk.
//
// Dijalankan dalam satu transaksi: kalau insert gagal di tengah, pemetaan
// lama tidak ikut hilang. Sebelumnya DELETE dan INSERT berdiri sendiri,
// jadi kegagalan di tengah meninggalkan produk tanpa add-on sama sekali.
func (r *ProductRepository) SyncProductAddOn(ctx context.Context, productID int64, addonIDs []int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM product_addon_map WHERE product_id = $1`, productID); err != nil {
		return err
	}

	for _, addonID := range addonIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO product_addon_map (product_id, product_addon_id) VALUES ($1, $2)`,
			productID, addonID,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// ============================================================================
// Product add-on
// ============================================================================

func (r *ProductAddOnRepository) CreateProductAddOn(ctx context.Context, p *entity.ProductAddon) error {
	const query = `
		INSERT INTO product_addons (name, price, stock, is_active)
		VALUES ($1, $2, $3, $4) RETURNING id`
	return r.db.QueryRow(ctx, query, p.Name, p.Price, p.Stock, p.IsActive).Scan(&p.ID)
}

func (r *ProductAddOnRepository) GetAllProductAddOn(ctx context.Context) ([]entity.ProductAddon, error) {
	const query = `SELECT id, name, price, stock, is_active FROM product_addons ORDER BY id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	addons := make([]entity.ProductAddon, 0, 8)
	for rows.Next() {
		var p entity.ProductAddon
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock, &p.IsActive); err != nil {
			return nil, err
		}
		addons = append(addons, p)
	}
	return addons, rows.Err()
}

func (r *ProductAddOnRepository) GetProductAddOnByID(ctx context.Context, id int64) (entity.ProductAddon, error) {
	const query = `SELECT id, name, price, stock, is_active FROM product_addons WHERE id = $1`

	var p entity.ProductAddon
	err := r.db.QueryRow(ctx, query, id).Scan(&p.ID, &p.Name, &p.Price, &p.Stock, &p.IsActive)
	if err != nil {
		if errors.Is(err, config.ErrNoRows) {
			return p, errProductNotFound
		}
		// BUG LAMA: baris ini `return p, nil` — error ditelan diam-diam,
		// dan pemanggil menerima struct kosong seolah sukses.
		return p, err
	}
	return p, nil
}

func (r *ProductAddOnRepository) UpdateProductAddOn(ctx context.Context, p *entity.ProductAddon) error {
	const query = `
		UPDATE product_addons
		SET name = $1, price = $2, stock = $3, is_active = $4
		WHERE id = $5`

	tag, err := r.db.Exec(ctx, query, p.Name, p.Price, p.Stock, p.IsActive, p.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errProductNotFound
	}
	return nil
}

func (r *ProductAddOnRepository) DeleteProductAddOn(ctx context.Context, id int64) error {
	const query = `DELETE FROM product_addons WHERE id = $1`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errProductNotFound
	}
	return nil
}