package http

import (
	"db/internal/models"
	"db/internal/repository"
	"log"

	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type ReservationGetByIDHandler struct {
	Repo *repository.ReservationRepository
}

func NewReservationGetByIDHandler(repo *repository.ReservationRepository) *ReservationGetByIDHandler {
	return &ReservationGetByIDHandler{
		Repo: repo,
	}
}
func (h *ReservationGetByIDHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if id == "" {
		http.Error(w, repository.ErrReservationNotExists.Error(), http.StatusNotFound)
		return
	}

	result, err := h.Repo.GetByID(
		r.Context(),
		id,
	)

	if err == repository.ErrReservationNotExists {
		http.Error(
			w,
			`{"error":"reservation_not_exists"}`,
			http.StatusNotFound,
		)
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
	json.NewEncoder(w).Encode(*result)
}

type ReservationCreateHandler struct {
	Repo *repository.ReservationRepository
}

func NewReservationCreateHandler(repo *repository.ReservationRepository) *ReservationCreateHandler {
	return &ReservationCreateHandler{
		Repo: repo,
	}
}
func (h *ReservationCreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var dto models.CreateReservationDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(
			w,
			`{"error":"invalid_json"}`,
			http.StatusBadRequest,
		)
		return
	}

	result, err := h.Repo.Create(r.Context(), dto)
	if errors.Is(err, repository.ErrProductNotFound) {
		http.Error(
			w,
			err.Error(),
			http.StatusNotFound,
		)
		return
	}
	if errors.Is(err, repository.ErrReservationAlreadyExists) {
		http.Error(
			w,
			err.Error(),
			http.StatusConflict,
		)
		return
	}
	if errors.Is(err, repository.ErrOutOfStock) {
		http.Error(
			w,
			err.Error(),
			http.StatusConflict,
		)
		return
	}
	if err != nil {
		log.Printf("Create reservation error: %v", err)

		http.Error(
			w,
			"internal_server_error",
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(*result)
}

type ReservationPayHandler struct {
	Repo *repository.ReservationRepository
}

func NewReservationPayHandler(
	repo *repository.ReservationRepository,
) *ReservationPayHandler {
	return &ReservationPayHandler{Repo: repo}
}

func (h *ReservationPayHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(
			w,
			`{"error":"reservation_not_exists"}`,
			http.StatusNotFound,
		)
		return
	}

	var dto models.PayReservationDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(
			w,
			`{"error":"invalid_json"}`,
			http.StatusBadRequest,
		)
		return
	}

	result, err := h.Repo.Pay(r.Context(), id, dto)

	switch {
	case errors.Is(err, repository.ErrReservationNotExists):
		http.Error(
			w,
			`{"error":"reservation_not_exists"}`,
			http.StatusNotFound,
		)
		return

	case errors.Is(err, repository.ErrInvalidPaymentState):
		http.Error(
			w,
			`{"error":"invalid_payment_state"}`,
			http.StatusConflict,
		)
		return

	case errors.Is(err, repository.ErrPaymentConflict):
		http.Error(
			w,
			`{"error":"payment_conflict"}`,
			http.StatusConflict,
		)
		return

	case errors.Is(err, repository.ErrAmountDismatch):
		http.Error(
			w,
			`{"error":"amount_mismatch"}`,
			http.StatusBadRequest,
		)
		return

	case err != nil:
		http.Error(
			w,
			`{"error":"internal_server_error"}`,
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		return
	}
}

type ReservationCancelHandler struct {
	Repo *repository.ReservationRepository
}

func NewReservationCancelHandler(
	repo *repository.ReservationRepository,
) *ReservationCancelHandler {
	return &ReservationCancelHandler{Repo: repo}
}

func (h *ReservationCancelHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(
			w,
			`{"error":"reservation_not_exists"}`,
			http.StatusNotFound,
		)
		return
	}

	var dto models.CancelReservationDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(
			w,
			`{"error":"invalid_json"}`,
			http.StatusBadRequest,
		)
		return
	}

	result, err := h.Repo.Cancel(r.Context(), id, dto)

	switch {
	case errors.Is(err, repository.ErrReservationNotExists):
		http.Error(
			w,
			`{"error":"reservation_not_exists"}`,
			http.StatusNotFound,
		)
		return

	case errors.Is(err, repository.ErrInvalidCancelState):
		http.Error(
			w,
			`{"error":"invalid_cancel_state"}`,
			http.StatusConflict,
		)
		return

	case err != nil:
		http.Error(
			w,
			`{"error":"internal_server_error"}`,
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		return
	}
}

type ReservationExpireHandler struct {
	Repo *repository.ReservationRepository
}

func NewReservationExpireHandler(
	repo *repository.ReservationRepository,
) *ReservationExpireHandler {
	return &ReservationExpireHandler{Repo: repo}
}

func (h *ReservationExpireHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	var dto struct {
		Limit *int `json:"limit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(
			w,
			`{"error":"invalid_json"}`,
			http.StatusBadRequest,
		)
		return
	}

	result, err := h.Repo.Expire(r.Context(), dto.Limit)

	switch {
	case errors.Is(err, repository.ErrLimitIsNotAllowed):
		http.Error(
			w,
			`{"error":"limit_not_allowed"}`,
			http.StatusBadRequest,
		)
		return

	case err != nil:
		http.Error(
			w,
			`{"error":"internal_server_error"}`,
			http.StatusInternalServerError,
		)
		return
	}

	type expireResponse struct {
		Items []models.ExpiredReservaionResponse `json:"items"`
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(expireResponse{Items: result}); err != nil {
		return
	}
}
