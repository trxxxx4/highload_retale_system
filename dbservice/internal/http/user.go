package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"db/internal/models"
	"db/internal/repository"

	"github.com/jackc/pgx/v5/pgconn"
)

type UserHandler struct {
	UserRepo *repository.UserRepository
}

func NewUserHandler(
	userRepo *repository.UserRepository,
) *UserHandler {
	return &UserHandler{
		UserRepo: userRepo,
	}
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var dto models.RegisterResponse

	err := json.NewDecoder(r.Body).Decode(&dto)
	if err != nil {
		http.Error(
			w,
			"invalid request body",
			http.StatusBadRequest,
		)
		return
	}

	if dto.ID == "" || dto.Email == "" {
		http.Error(
			w,
			"id and email are required",
			http.StatusBadRequest,
		)
		return
	}

	err = h.UserRepo.Create(r.Context(), dto)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(
				w,
				"user already exists",
				http.StatusConflict,
			)
			return
		}

		http.Error(
			w,
			"failed to create user",
			http.StatusInternalServerError,
		)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
