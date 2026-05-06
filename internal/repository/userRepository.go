package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"coffeeshop/internal/entity"

	
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*entity.User, error) {
	query := `SELECT id, email, username, password, role, is_active 
	          FROM users WHERE email = $1`
	
	var user entity.User

	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.Password,
		&user.Role,
		&user.IsActive,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
