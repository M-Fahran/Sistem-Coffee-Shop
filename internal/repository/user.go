package repository

import (
	"coffeeshop/internal/entity"
	"context"
	"database/sql"
	"errors"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	query := `
			SELECT id, email, username, password, role, is_active, 
			FROM users WHERE email = $1
	`

	var user entity.User
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.Password,
		&user.Role,
		&user.IsActive,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user tidak ditemukan")
		}
		return nil, err
	}
	return &user, nil
}
