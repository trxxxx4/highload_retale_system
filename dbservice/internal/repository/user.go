package repository

import (
	"context"
	"fmt"

	"db/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	Pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		Pool: pool,
	}
}

func (repo *UserRepository) Create(
	ctx context.Context,
	dto models.RegisterResponse,
) error {
	query := `
		INSERT INTO users (id, email)
		VALUES ($1, $2);
	`

	_, err := repo.Pool.Exec(
		ctx,
		query,
		dto.ID,
		dto.Email,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}
