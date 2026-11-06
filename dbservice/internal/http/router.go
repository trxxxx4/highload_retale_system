package http

import (
	"db/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(pool *pgxpool.Pool) *chi.Mux {
	r := chi.NewRouter()

	handlerReadyz := NewReadyHandler(pool)

	handlerProduct := NewProductHandler(
		repository.NewProductRepository(pool),
	)

	reservationRepo := repository.NewReservationRepository(pool)

	reservationGetHandler := NewReservationGetByIDHandler(
		reservationRepo,
	)
	reservationCreateHandler := NewReservationCreateHandler(
		reservationRepo,
	)
	reservationPayHandler := NewReservationPayHandler(
		reservationRepo,
	)
	reservationCancelHandler := NewReservationCancelHandler(
		reservationRepo,
	)
	reservationExpireHandler := NewReservationExpireHandler(
		reservationRepo,
	)

	userHandler := NewUserHandler(repository.NewUserRepository(pool))
	stockRepo := repository.NewStockRepository(pool)
	stockListHandler := NewStockListHandler(stockRepo)
	stockHandler := NewStockItemHandler(stockRepo)

	r.Get("/readyz", handlerReadyz.ServeHTTP)
	r.Get("/healthz", HandleHealth)

	r.Get("/internal/products", handlerProduct.ServeHTTP)

	r.Get("/internal/stock", stockListHandler.ServeHTTP)
	r.Get("/internal/stock/{sku}", stockHandler.ServeHTTP)

	r.Get("/internal/reservations/{id}", reservationGetHandler.ServeHTTP)
	r.Post("/internal/reservations", reservationCreateHandler.ServeHTTP)
	r.Post("/internal/reservations/{id}/pay", reservationPayHandler.ServeHTTP)
	r.Post("/internal/reservations/{id}/cancel", reservationCancelHandler.ServeHTTP)

	r.Post("/internal/reservations/expire", reservationExpireHandler.ServeHTTP)
	r.Post("/internal/users", userHandler.Create)
	return r
}
