package http

import (
	"db/internal/models"
	"db/internal/repository"
	"encoding/json"
	"net/http"
	"strconv"
)

type ProductHandler struct {
	ProductRepo *repository.ProductRepository
}

func NewProductHandler(
	productRepo *repository.ProductRepository,
) *ProductHandler {
	return &ProductHandler{
		ProductRepo: productRepo,
	}
}

func (h *ProductHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit := 50
	offset := 0

	parsedLimit, err := strconv.Atoi(query.Get("limit"))
	if err != nil && query.Get("limit") != "" {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	parsedOffset, err := strconv.Atoi(query.Get("offset"))
	if err != nil && query.Get("offset") != "" {
		http.Error(w, "invalid offset", http.StatusBadRequest)
		return
	}
	if parsedLimit < 1 || parsedLimit > 200 {
		http.Error(
			w,
			"invalid limit",
			http.StatusBadRequest,
		)
		return
	}
	if query.Get("limit") == "" {
		parsedLimit = limit
		parsedOffset = offset
	}
	result, err := h.ProductRepo.GetAll(r.Context(), parsedLimit, parsedOffset)
	if err != nil {
		http.Error(
			w,
			"database request failure",
			http.StatusInternalServerError,
		)
		return
	}

	type productsResponse struct {
		Items []models.Product `json:"items"`
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(productsResponse{Items: result}); err != nil {
		return
	}

}
