package repository

import (
	"auth/internal/models"
	"auth/internal/service"
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRegisterRepository struct {
	Pool *pgxpool.Pool
}

func NewUserRegisterRepository(pool *pgxpool.Pool) *UserRegisterRepository {
	return &UserRegisterRepository{
		Pool: pool,
	}
}

func (repo *UserRegisterRepository) Registration(ctx context.Context, req models.CredentialsRequest) (*models.RegisterResponse, error) {

	if len(req.Password) < 8 || len(req.Password) > 72 {
		return nil, ErrorBadRequest
	}
	hashPassword, err := service.HashPassword(req.Password)
	if err != nil {
		return nil, ErrorInternalServerError
	}
	query := `
	INSERT INTO users (email, password_hash, created_at)
	VALUES ($1, $2, now())
	ON CONFLICT DO NOTHING
	RETURNING id, email;
	`
	email := strings.ToLower(req.Email)
	var responseDTO models.RegisterResponse
	err = repo.Pool.QueryRow(ctx, query, email, hashPassword).Scan(&responseDTO.ID, &responseDTO.Email)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrorConflict
	}
	return &responseDTO, nil

}

type UserLoginRepository struct {
	Pool *pgxpool.Pool
}

func NewUserLoginRepository(
	pool *pgxpool.Pool,
) *UserLoginRepository {
	return &UserLoginRepository{
		Pool: pool,
	}
}

func (repo *UserLoginRepository) GetByEmail(
	ctx context.Context,
	email string,
) (*models.UserAuth, error) {
	query := `
		SELECT id, email, password_hash
		FROM users
		WHERE email = $1;
	`

	email = strings.ToLower(strings.TrimSpace(email))

	var user models.UserAuth

	err := repo.Pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}

		return nil, err
	}

	return &user, nil
}
