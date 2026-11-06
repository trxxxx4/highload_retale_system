package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"inventory/internal/http/dbclient"
	"inventory/internal/http/middleware"
	redisCache "inventory/internal/redis"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

type ReservationsHandler struct {
	DBClient *dbclient.Client
	Redis    *goredis.Client
}

func NewReservationsHandler(
	dbClient *dbclient.Client,
	redisClient *goredis.Client,
) *ReservationsHandler {
	return &ReservationsHandler{
		DBClient: dbClient,
		Redis:    redisClient,
	}
}

type createReservationRequest struct {
	SKU string `json:"sku"`
	Qty int    `json:"qty"`
}

type payReservationRequest struct {
	AmountCents int `json:"amount_cents"`
}

type stockResponse struct {
	SKU       string `json:"sku"`
	OnHand    int    `json:"on_hand"`
	Reserved  int    `json:"reserved"`
	Available int    `json:"available"`
}

// Create обрабатывает POST /api/reservations.
func (h *ReservationsHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input createReservationRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	input.SKU = strings.TrimSpace(input.SKU)

	if input.SKU == "" || input.Qty <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	// Если Redis содержит значение, меньшее запрошенного
	// количества, сразу отказываем в бронировании.
	//
	// Если ключа нет или Redis недоступен, проверяем
	// остаток через db-service.
	cachedAvailable, cacheErr := h.readCachedStock(
		ctx,
		input.SKU,
	)

	if cacheErr == nil {
		if cachedAvailable < input.Qty {
			writeError(
				w,
				http.StatusConflict,
				"out_of_stock",
			)
			return
		}
	} else {
		if !errors.Is(cacheErr, goredis.Nil) {
			log.Printf(
				"Redis read failed for SKU %s: %v",
				input.SKU,
				cacheErr,
			)
		}

		stock, ok := h.fetchStock(ctx, w, input.SKU)
		if !ok {
			return
		}

		if err := h.writeCachedStock(
			ctx,
			input.SKU,
			stock.Available,
			true,
		); err != nil {
			log.Printf(
				"Redis cache fill failed for SKU %s: %v",
				input.SKU,
				err,
			)
		}

		if stock.Available < input.Qty {
			writeError(
				w,
				http.StatusConflict,
				"out_of_stock",
			)
			return
		}
	}

	// PostgreSQL остаётся источником истины.
	// db-service должен атомарно проверить остаток.
	request := dbclient.CreateReservationRequest{
		ID:             uuid.New(),
		UserID:         userID,
		SKU:            input.SKU,
		Qty:            input.Qty,
		IdempotencyKey: uuid.NewString(),
		TTLSeconds:     900,
	}

	status, headers, body, err := h.DBClient.RequestJSON(
		ctx,
		http.MethodPost,
		"/internal/reservations",
		request,
	)
	if err != nil {
		writeUnavailable(w, err)
		return
	}

	if status < 200 || status >= 300 {
		writeDBResponse(w, status, headers, body)
		return
	}

	// Бронь уже создана. Обновление Redis не должно
	// превращать успешную бронь в ошибку.
	h.refreshStockCache(ctx, input.SKU)

	writeDBResponse(w, status, headers, body)
}

// Get обрабатывает GET /api/reservations/{id}.
func (h *ReservationsHandler) Get(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	reservationID, ok := parseReservationID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_reservation_id")
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	status, headers, body, err := h.DBClient.RequestJSON(
		ctx,
		http.MethodGet,
		"/internal/reservations/"+reservationID.String(),
		nil,
	)
	if err != nil {
		writeUnavailable(w, err)
		return
	}

	if status < 200 || status >= 300 {
		writeDBResponse(w, status, headers, body)
		return
	}

	var reservation dbclient.Reservation
	if err := json.Unmarshal(body, &reservation); err != nil {
		writeError(
			w,
			http.StatusBadGateway,
			"invalid_db_service_response",
		)
		return
	}

	// Нельзя раскрывать существование чужой брони.
	if reservation.UserID != userID {
		writeError(w, http.StatusNotFound, "reservation_not_found")
		return
	}

	writeDBResponse(w, status, headers, body)
}

// Pay обрабатывает POST /api/reservations/{id}/pay.
func (h *ReservationsHandler) Pay(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	reservationID, ok := parseReservationID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_reservation_id")
		return
	}

	var input payReservationRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	if input.AmountCents < 0 {
		writeError(w, http.StatusBadRequest, "invalid_amount")
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	reservation, status, headers, body, ok := h.getOwnedReservation(
		ctx,
		w,
		reservationID,
		userID,
	)
	if !ok {
		return
	}

	_ = reservation

	// Сохраняем контракт существующего db-service:
	// передаём payment_id и amount_cents.
	paymentRequest := struct {
		PaymentID   string `json:"payment_id"`
		AmountCents int    `json:"amount_cents"`
	}{
		PaymentID:   uuid.NewString(),
		AmountCents: input.AmountCents,
	}

	status, headers, body, err := h.DBClient.RequestJSON(
		ctx,
		http.MethodPost,
		"/internal/reservations/"+reservationID.String()+"/pay",
		paymentRequest,
	)
	if err != nil {
		writeUnavailable(w, err)
		return
	}

	if status < 200 || status >= 300 {
		writeDBResponse(w, status, headers, body)
		return
	}

	// Синхронизируем кэш с PostgreSQL после успешной оплаты.
	// Если остаток не изменился, будет записано то же значение.
	h.refreshStockCache(ctx, reservation.SKU)

	writeDBResponse(w, status, headers, body)
}

// Cancel обрабатывает DELETE /api/reservations/{id}.
func (h *ReservationsHandler) Cancel(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodDelete {
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		writeError(
			w,
			http.StatusUnauthorized,
			"unauthorized",
		)
		return
	}

	reservationID, ok := parseReservationID(r)
	if !ok {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid_reservation_id",
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	reservation, err := h.DBClient.Cancel(
		ctx,
		reservationID,
		userID,
	)
	if err != nil {
		writeUnavailable(w, err)
		return
	}

	// Получаем фактический остаток после отмены из PostgreSQL.
	h.refreshStockCache(ctx, reservation.SKU)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(reservation)
}

// getOwnedReservation получает бронь и проверяет владельца.
// При ошибке уже пишет соответствующий ответ клиенту.
func (h *ReservationsHandler) getOwnedReservation(
	ctx context.Context,
	w http.ResponseWriter,
	id uuid.UUID,
	userID uuid.UUID,
) (dbclient.Reservation, int, http.Header, []byte, bool) {
	var empty dbclient.Reservation

	status, headers, body, err := h.DBClient.RequestJSON(
		ctx,
		http.MethodGet,
		"/internal/reservations/"+id.String(),
		nil,
	)
	if err != nil {
		writeUnavailable(w, err)
		return empty, 0, nil, nil, false
	}

	if status < 200 || status >= 300 {
		writeDBResponse(w, status, headers, body)
		return empty, status, headers, body, false
	}

	var reservation dbclient.Reservation
	if err := json.Unmarshal(body, &reservation); err != nil {
		writeError(
			w,
			http.StatusBadGateway,
			"invalid_db_service_response",
		)
		return empty, 0, nil, nil, false
	}

	if reservation.UserID != userID {
		writeError(w, http.StatusNotFound, "reservation_not_found")
		return empty, 0, nil, nil, false
	}

	return reservation, status, headers, body, true
}

// fetchStock получает остаток через db-service.
// При ошибке пишет ответ и возвращает false.
func (h *ReservationsHandler) fetchStock(
	ctx context.Context,
	w http.ResponseWriter,
	sku string,
) (stockResponse, bool) {
	var stock stockResponse

	path := "/internal/stock/" + url.PathEscape(sku)

	status, headers, body, err := h.DBClient.RequestJSON(
		ctx,
		http.MethodGet,
		path,
		nil,
	)
	if err != nil {
		writeUnavailable(w, err)
		return stock, false
	}

	if status < 200 || status >= 300 {
		writeDBResponse(w, status, headers, body)
		return stock, false
	}

	if err := json.Unmarshal(body, &stock); err != nil {
		writeError(
			w,
			http.StatusBadGateway,
			"invalid_db_service_response",
		)
		return stock, false
	}

	return stock, true
}

// readCachedStock читает остаток из Redis.
func (h *ReservationsHandler) readCachedStock(
	ctx context.Context,
	sku string,
) (int, error) {
	if h.Redis == nil {
		return 0, errors.New("Redis client is nil")
	}

	return redisCache.Stock(ctx, h.Redis, sku)
}

// writeCachedStock записывает остаток в Redis.
// Если onlyIfAbsent=true, существующее значение не меняется.
func (h *ReservationsHandler) writeCachedStock(
	ctx context.Context,
	sku string,
	available int,
	onlyIfAbsent bool,
) error {
	if h.Redis == nil {
		return errors.New("Redis client is nil")
	}

	if onlyIfAbsent {
		return redisCache.SetStockIfAbsent(
			ctx,
			h.Redis,
			sku,
			available,
		)
	}

	return redisCache.SetStock(
		ctx,
		h.Redis,
		sku,
		available,
	)
}

// refreshStockCache получает актуальное значение из db-service
// и записывает его в Redis. Ошибка не меняет результат операции.
func (h *ReservationsHandler) refreshStockCache(
	ctx context.Context,
	sku string,
) {
	if h.Redis == nil {
		log.Printf("Redis client is nil; cannot refresh SKU %s", sku)
		return
	}

	stock, err := h.DBClient.Stock(ctx, sku)
	if err != nil {
		log.Printf("failed to refresh stock for SKU %s: %v", sku, err)
		return
	}

	if err := redisCache.SetStock(
		ctx,
		h.Redis,
		sku,
		stock.Available,
	); err != nil {
		log.Printf("failed to update cached stock for SKU %s: %v", sku, err)
	}
}

func getUserID(r *http.Request) (uuid.UUID, bool) {
	value, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return uuid.Nil, false
	}

	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, false
	}

	return id, true
}

func parseReservationID(r *http.Request) (uuid.UUID, bool) {
	value := chi.URLParam(r, "id")
	id, err := uuid.Parse(value)
	return id, err == nil
}

// writeDBResponse передаёт клиенту статус, заголовки и тело db-service.
func writeDBResponse(
	w http.ResponseWriter,
	status int,
	headers http.Header,
	body []byte,
) {
	for key, values := range headers {
		if isHopByHopHeader(key) {
			continue
		}

		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func isHopByHopHeader(name string) bool {
	switch strings.ToLower(name) {
	case "connection",
		"keep-alive",
		"proxy-authenticate",
		"proxy-authorization",
		"te",
		"trailer",
		"transfer-encoding",
		"upgrade":
		return true
	default:
		return false
	}
}

func writeUnavailable(w http.ResponseWriter, err error) {
	log.Printf("db-service request failed: %v", err)

	writeError(
		w,
		http.StatusServiceUnavailable,
		"service_unavailable",
	)
}

func writeError(
	w http.ResponseWriter,
	status int,
	code string,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": code,
	})
}

// Оставляем strconv в этом файле доступным для расширения
// формата ответов без дополнительных изменений импортов.
var _ = strconv.Itoa

var _ = fmt.Sprintf
