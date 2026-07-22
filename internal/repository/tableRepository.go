package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"coffeeshop/internal/config"
	"coffeeshop/internal/entity"
	"coffeeshop/internal/support/exception"
)

type TableRepository struct {
	pool *config.Pool
}

func NewTableRepository(pool *config.Pool) *TableRepository {
	return &TableRepository{pool: pool}
}

// ============================================================================
// CREATE
// ============================================================================

func (r *TableRepository) Create(ctx context.Context, t *entity.Table) error {
	const query = `
		INSERT INTO tables (number, qr_token, is_active)
		VALUES ($1, $2, $3)
		RETURNING id`

	err := r.pool.QueryRow(ctx, query, t.Number, t.QRToken, t.IsActive).Scan(&t.ID)
	if err != nil {
		var pgErr *config.PgError
		if errors.As(err, &pgErr) && pgErr.Code == config.PgErrUniqueViolation {
			if strings.Contains(pgErr.ConstraintName, "number") {
				return exception.Conflict("TABLE_409", "table number already exists")
			}
			if strings.Contains(pgErr.ConstraintName, "qr_token") {
				return exception.Conflict("TABLE_409", "QR token collision, please retry")
			}
		}
		return exception.Internal(fmt.Errorf("table create: %w", err))
	}
	return nil
}

// ============================================================================
// READ
// ============================================================================

func (r *TableRepository) FindByID(ctx context.Context, id int64) (*entity.Table, error) {
	const query = `
		SELECT id, number, qr_token, is_active
		FROM tables WHERE id = $1`

	var t entity.Table
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&t.ID, &t.Number, &t.QRToken, &t.IsActive,
	)
	if err != nil {
		if errors.Is(err, config.ErrNoRows) {
			return nil, exception.NotFound("TABLE_404", "table not found")
		}
		return nil, exception.Internal(fmt.Errorf("table find by id: %w", err))
	}
	return &t, nil
}

func (r *TableRepository) FindActiveByQRToken(ctx context.Context, token string) (*entity.Table, error) {
	const query = `
		SELECT id, number, qr_token, is_active
		FROM tables 
		WHERE qr_token = $1 AND is_active = TRUE`

	var t entity.Table
	err := r.pool.QueryRow(ctx, query, token).Scan(
		&t.ID, &t.Number, &t.QRToken, &t.IsActive,
	)
	if err != nil {
		if errors.Is(err, config.ErrNoRows) {
			return nil, exception.NotFound("TABLE_404", "table not found or inactive")
		}
		return nil, exception.Internal(fmt.Errorf("table find by qr token: %w", err))
	}
	return &t, nil
}

func (r *TableRepository) List(ctx context.Context, in entity.ListTablesInput) ([]entity.Table, int, error) {
	var (
		conditions []string
		args       []any
		argIdx     = 1
	)

	if in.Search != "" {
		conditions = append(conditions, fmt.Sprintf("number ILIKE $%d", argIdx))
		args = append(args, "%"+in.Search+"%")
		argIdx++
	}
	if in.IsActive != nil {
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", argIdx))
		args = append(args, *in.IsActive)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count first
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM tables %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, exception.Internal(fmt.Errorf("table count: %w", err))
	}
	if total == 0 {
		return []entity.Table{}, 0, nil
	}

	// Then data
	offset := (in.Page - 1) * in.PerPage
	dataQuery := fmt.Sprintf(`
		SELECT id, number, qr_token, is_active
		FROM tables %s
		ORDER BY number ASC
		LIMIT $%d OFFSET $%d`, whereClause, argIdx, argIdx+1)
	args = append(args, in.PerPage, offset)

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, exception.Internal(fmt.Errorf("table list query: %w", err))
	}
	defer rows.Close()

	tables := make([]entity.Table, 0, in.PerPage)
	for rows.Next() {
		var t entity.Table
		if err := rows.Scan(&t.ID, &t.Number, &t.QRToken, &t.IsActive); err != nil {
			return nil, 0, exception.Internal(fmt.Errorf("table list scan: %w", err))
		}
		tables = append(tables, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, exception.Internal(fmt.Errorf("table list rows: %w", err))
	}

	return tables, total, nil
}

// ============================================================================
// UPDATE
// ============================================================================

func (r *TableRepository) Update(ctx context.Context, t *entity.Table) error {
	const query = `
		UPDATE tables 
		SET number = $1, is_active = $2
		WHERE id = $3`

	tag, err := r.pool.Exec(ctx, query, t.Number, t.IsActive, t.ID)
	if err != nil {
		var pgErr *config.PgError
		if errors.As(err, &pgErr) && pgErr.Code == config.PgErrUniqueViolation {
			return exception.Conflict("TABLE_409", "table number already exists")
		}
		return exception.Internal(fmt.Errorf("table update: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return exception.NotFound("TABLE_404", "table not found")
	}
	return nil
}

func (r *TableRepository) UpdateQRToken(ctx context.Context, id int64, token string) (*entity.Table, error) {
	const query = `
		UPDATE tables 
		SET qr_token = $1
		WHERE id = $2
		RETURNING id, number, qr_token, is_active`

	var t entity.Table
	err := r.pool.QueryRow(ctx, query, token, id).Scan(
		&t.ID, &t.Number, &t.QRToken, &t.IsActive,
	)
	if err != nil {
		if errors.Is(err, config.ErrNoRows) {
			return nil, exception.NotFound("TABLE_404", "table not found")
		}
		var pgErr *config.PgError
		if errors.As(err, &pgErr) && pgErr.Code == config.PgErrUniqueViolation {
			return nil, exception.Conflict("TABLE_409", "QR token collision, please retry")
		}
		return nil, exception.Internal(fmt.Errorf("table update qr_token: %w", err))
	}
	return &t, nil
}

func (r *TableRepository) ToggleActive(ctx context.Context, id int64) (*entity.Table, error) {
	const query = `
		UPDATE tables 
		SET is_active = NOT is_active
		WHERE id = $1
		RETURNING id, number, qr_token, is_active`

	var t entity.Table
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&t.ID, &t.Number, &t.QRToken, &t.IsActive,
	)
	if err != nil {
		if errors.Is(err, config.ErrNoRows) {
			return nil, exception.NotFound("TABLE_404", "table not found")
		}
		return nil, exception.Internal(fmt.Errorf("table toggle active: %w", err))
	}
	return &t, nil
}

// ============================================================================
// DELETE
// ============================================================================

func (r *TableRepository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tables WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		var pgErr *config.PgError
		if errors.As(err, &pgErr) && pgErr.Code == config.PgErrForeignKeyViolation {
			return exception.Conflict("TABLE_409", "table is referenced by existing orders")
		}
		return exception.Internal(fmt.Errorf("table delete: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return exception.NotFound("TABLE_404", "table not found")
	}
	return nil
}

func (r *TableRepository) BulkDelete(ctx context.Context, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return []int64{}, nil
	}

	const query = `
		DELETE FROM tables 
		WHERE id = ANY($1)
		RETURNING id`

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	rows, err := r.pool.Query(ctx, query, ids)
	if err != nil {
		var pgErr *config.PgError
		if errors.As(err, &pgErr) && pgErr.Code == config.PgErrForeignKeyViolation {
			return nil, exception.Conflict("TABLE_409", "one or more tables are referenced by existing orders")
		}
		return nil, exception.Internal(fmt.Errorf("table bulk delete: %w", err))
	}
	defer rows.Close()

	deleted := make([]int64, 0, len(ids))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, exception.Internal(fmt.Errorf("table bulk delete scan: %w", err))
		}
		deleted = append(deleted, id)
	}
	if err := rows.Err(); err != nil {
		return nil, exception.Internal(fmt.Errorf("table bulk delete rows: %w", err))
	}
	return deleted, nil
}