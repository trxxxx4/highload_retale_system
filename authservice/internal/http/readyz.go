package http

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReadyzHandler struct {
	Pool           *pgxpool.Pool
	MigrateVersion int
}

func NewReadyHandler(pool *pgxpool.Pool, migrateVersion int) *ReadyzHandler {
	return &ReadyzHandler{
		Pool: pool, MigrateVersion: migrateVersion,
	}
}

func (h *ReadyzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := h.Pool.Ping(ctx)
	if err != nil {
		http.Error(
			w,
			"service_is_not_ready",
			http.StatusServiceUnavailable,
		)
		return
	}

	query := "SELECT version, dirty FROM schema_migrations"
	var version int64
	var dirty bool
	err = h.Pool.QueryRow(ctx, query).Scan(&version, &dirty)
	if err != nil || dirty || version != int64(h.MigrateVersion) {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}
