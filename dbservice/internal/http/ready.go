package http

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReadyHandler struct {
	Pool *pgxpool.Pool
}

func NewReadyHandler(pool *pgxpool.Pool) *ReadyHandler {
	return &ReadyHandler{Pool: pool}
}

const expectVersion int64 = 1

func (h *ReadyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(
		r.Context(),
		time.Second,
	)
	defer cancel()
	if err := h.Pool.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	query := "SELECT version, dirty FROM schema_migrations"
	var version int64
	var dirty bool
	err := h.Pool.QueryRow(ctx, query).Scan(&version, &dirty)
	if err != nil || dirty || version != expectVersion {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)

}
