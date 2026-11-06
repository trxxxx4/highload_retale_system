package http

import (
	"db/internal/models"
	"db/internal/repository"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type StockItemHandler struct {
	Repo *repository.StockRepository
}

func NewStockItemHandler(
	repo *repository.StockRepository,
) *StockItemHandler {
	return &StockItemHandler{
		Repo: repo,
	}
}

func (h *StockItemHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sku := chi.URLParam(r, "sku")

	if sku == "" {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
		return
	}

	result, err := h.Repo.GetBySKU(
		r.Context(),
		sku,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(
			w,
			`{"error":"internal_server_error"}`,
			http.StatusInternalServerError,
		)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)

}

type StockListHandler struct {
	Repo *repository.StockRepository
}

func NewStockListHandler(
	repo *repository.StockRepository,
) *StockListHandler {
	return &StockListHandler{
		Repo: repo,
	}
}

func (h *StockListHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	query := r.URL.Query()

	after := query.Get("after")
	limit := 1000

	if query.Has("limit") {
		parsedLimit, err := strconv.Atoi(query.Get("limit"))
		if err != nil || parsedLimit < 1 || parsedLimit > 5000 {
			http.Error(
				w,
				`invalid_limit`,
				http.StatusBadRequest,
			)
			return
		}
		limit = parsedLimit
	}

	items, err := h.Repo.GetAfter(
		r.Context(),
		after,
		limit+1,
	)
	if err != nil {
		http.Error(
			w,
			`{"error":"internal_server_error"}`,
			http.StatusInternalServerError,
		)
		return
	}

	nextAfter := ""

	if len(items) > limit {
		items = items[:limit]
		nextAfter = items[len(items)-1].SKU
	}

	response := models.ResponseStock{
		Items:     items,
		NextAfter: nextAfter,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
