package api

import (
	"context"
	"encoding/json"
	"inventory/internal/http/dbclient"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type ProductsHandler struct {
	DBClient *dbclient.Client
}

func NewProductsHandler(clientPtr *dbclient.Client) *ProductsHandler {
	return &ProductsHandler{

		DBClient: clientPtr,
	}
}

func (h *ProductsHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {

	ctx, cancel := context.WithTimeout(
		r.Context(),
		3*time.Second,
	)
	defer cancel()

	query := r.URL.Query()

	limit := 50
	offset := 0

	if value := query.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 200 {
			http.Error(
				w,
				"invalid limit",
				http.StatusBadRequest,
			)
			return
		}
		limit = parsed
	}

	if value := query.Get("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			http.Error(
				w,
				"invalid offset",
				http.StatusBadRequest,
			)
			return
		}
		offset = parsed
	}

	resp, err := h.DBClient.Products(ctx, limit, offset)
	if err != nil {
		slog.Error("products request failed", "err", err, "limit", limit, "offset", offset)
		http.Error(w, "failed to get products", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return
	}

}
