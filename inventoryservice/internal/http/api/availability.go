package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"inventory/internal/http/dbclient"
	redisCache "inventory/internal/redis"

	"github.com/go-chi/chi/v5"
	goredis "github.com/redis/go-redis/v9"
)

type AvailabilityHandler struct {
	DBClient *dbclient.Client
	Redis    *goredis.Client
}

func NewAvailabilityHandler(
	dbClient *dbclient.Client,
	redisClient *goredis.Client,
) *AvailabilityHandler {
	return &AvailabilityHandler{
		DBClient: dbClient,
		Redis:    redisClient,
	}
}

type availabilityResponse struct {
	SKU       string `json:"sku"`
	Available int    `json:"available"`
}

func (h *AvailabilityHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	sku := chi.URLParam(r, "sku")
	if sku == "" {
		http.Error(
			w,
			"sku is required",
			http.StatusBadRequest,
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		3*time.Second,
	)
	defer cancel()

	// Redis — только кэш. При отсутствии ключа или сбое
	// переходим к PostgreSQL через db-service.
	available, err := redisCache.Stock(
		ctx,
		h.Redis,
		sku,
	)

	if err == nil {
		h.writeResponse(w, sku, available)
		return
	}

	if !errors.Is(err, goredis.Nil) {
		log.Printf("stock cache read failed for %s: %v", sku, err)
	}

	stock, err := h.DBClient.Stock(ctx, sku)
	if err != nil {
		if errors.Is(err, dbclient.ErrNotFound) {
			http.Error(
				w,
				"product not found",
				http.StatusNotFound,
			)
			return
		}

		http.Error(
			w,
			"failed to get stock",
			http.StatusBadGateway,
		)
		return
	}

	available = stock.Available

	// Ошибка записи в Redis не должна ломать запрос.
	// SetNX не перезапишет значение, которое мог записать
	// параллельный запрос.
	if err := redisCache.SetStockIfAbsent(
		ctx,
		h.Redis,
		sku,
		available,
	); err != nil {
		log.Printf("stock cache write failed for %s: %v", sku, err)
	}

	h.writeResponse(w, sku, available)
}

func (h *AvailabilityHandler) writeResponse(
	w http.ResponseWriter,
	sku string,
	available int,
) {
	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(
		availabilityResponse{
			SKU:       sku,
			Available: available,
		},
	)
}
