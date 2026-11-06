package http

import (
	"context"
	"errors"
	"inventory/internal/http/dbclient"
	"net/http"
	"time"
)

type ReadyzHandler struct {
	DBClient *dbclient.Client
}

func NewReadyzHandler(client *dbclient.Client) *ReadyzHandler {
	return &ReadyzHandler{

		DBClient: client,
	}
}

func (h *ReadyzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	err := h.DBClient.Ready(ctx)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			http.Error(
				w,
				"db-service timeout",
				http.StatusGatewayTimeout,
			)
			return
		}

		if r.Context().Err() != nil {
			return
		}

		http.Error(
			w,
			"db-service unavailable",
			http.StatusServiceUnavailable,
		)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready\n"))
}
