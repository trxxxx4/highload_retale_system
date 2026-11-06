package http

import (
	dbclient "auth/internal/dbservice"
	"auth/internal/repository"
	"auth/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(pool *pgxpool.Pool, migrateVersion int, JWTManager *service.JWTManager, clientDB *dbclient.Client) *chi.Mux {
	r := chi.NewRouter()
	UserRegisterRepository := repository.NewUserRegisterRepository(pool)
	UserLoginRepository := repository.NewUserLoginRepository(pool)
	ReadyzHandler := NewReadyHandler(pool, migrateVersion)
	UsersHandler := NewUsersHandler(UserRegisterRepository, UserLoginRepository, JWTManager, clientDB)
	r.Get("/healthz", HealthHandler)
	r.Get("/readyz", ReadyzHandler.ServeHTTP)
	r.Post("/api/auth/register", UsersHandler.Register)
	r.Post("/api/auth/login", UsersHandler.Login)

	return r
}
