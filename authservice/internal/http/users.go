package http

import (
	dbclient "auth/internal/dbservice"
	"auth/internal/models"
	"auth/internal/repository"
	"auth/internal/service"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

type UsersHandler struct {
	RegisterRepo *repository.UserRegisterRepository
	LoginRepo    *repository.UserLoginRepository
	JWT          *service.JWTManager
	ClientDB     *dbclient.Client
}

func NewUsersHandler(
	registerRepo *repository.UserRegisterRepository,
	loginRepo *repository.UserLoginRepository,
	jwtManager *service.JWTManager,
	clientDB *dbclient.Client,
) *UsersHandler {
	return &UsersHandler{
		RegisterRepo: registerRepo,
		LoginRepo:    loginRepo,
		JWT:          jwtManager,
		ClientDB:     clientDB,
	}
}

func (h *UsersHandler) Register(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req models.CredentialsRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.Email == "" || len(req.Password) < 8 ||
		len(req.Password) > 72 {
		http.Error(w, `{"error":"validation_error"}`,
			http.StatusBadRequest)
		return
	}

	user, err := h.RegisterRepo.Registration(r.Context(), req)
	if err != nil {
		if errors.Is(err, repository.ErrorConflict) {
			http.Error(w, `{"error":"email_taken"}`,
				http.StatusConflict)
			return
		}

		if errors.Is(err, repository.ErrorBadRequest) {
			http.Error(w, `{"error":"validation_error"}`,
				http.StatusBadRequest)
			return
		}

		http.Error(w, `{"error":"internal_server_error"}`,
			http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = h.ClientDB.AddUser(context.Background(), *user)
	if err != nil {
		log.Printf("add user in dbservice's table user error: %v", err)
	}
	if err := json.NewEncoder(w).Encode(user); err != nil {
		return
	}
}

func (h *UsersHandler) Login(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req models.CredentialsRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`,
			http.StatusBadRequest)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"invalid_credentials"}`,
			http.StatusUnauthorized)
		return
	}

	user, err := h.LoginRepo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, `{"error":"invalid_credentials"}`,
				http.StatusUnauthorized)
			return
		}

		http.Error(w, `{"error":"internal_server_error"}`,
			http.StatusInternalServerError)
		return
	}

	if err := service.CheckPassword(
		user.PasswordHash,
		req.Password,
	); err != nil {
		http.Error(w, `{"error":"invalid_credentials"}`,
			http.StatusUnauthorized)
		return
	}

	accessToken, err := h.JWT.GenerateToken(user.ID)
	if err != nil {
		http.Error(
			w,
			`{"error":"internal_server_error"}`,
			http.StatusInternalServerError,
		)
		return
	}

	response := models.LoginResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   h.JWT.GetTTL(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}
