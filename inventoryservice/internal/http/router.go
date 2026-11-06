package http

import (
	"inventory/internal/http/api"
	"inventory/internal/http/dbclient"
	"inventory/internal/http/middleware"
	"net/http"

	"github.com/go-chi/chi/v5"
	goredis "github.com/redis/go-redis/v9"
)

func NewRouter(
	auth *middleware.AuthMiddleware,
	clientDB *dbclient.Client,
	clientRedis *goredis.Client,
) http.Handler {
	r := chi.NewRouter()

	readyzHandler := NewReadyzHandler(clientDB)
	productsHandler := api.NewProductsHandler(clientDB)
	reservationsHandler := api.NewReservationsHandler(clientDB, clientRedis)
	availabilityHandler := api.NewAvailabilityHandler(
		clientDB,
		clientRedis,
	)

	r.Get("/healthz", HandleHealth)
	r.Get("/readyz", readyzHandler.ServeHTTP)

	r.Get("/api/products", productsHandler.ServeHTTP)

	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware)

		r.Get(
			"/api/products/{sku}/availability",
			availabilityHandler.ServeHTTP,
		)
		r.Post("/api/reservations", reservationsHandler.Create)
		r.Get("/api/reservations/{id}", reservationsHandler.Get)
		r.Post("/api/reservations/{id}/pay", reservationsHandler.Pay)
		r.Delete("/api/reservations/{id}", reservationsHandler.Cancel)
	})

	return r
}
