package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey string

const userIDKey contextKey = "user_id"

type AuthMiddleware struct {
	secret []byte
	issuer string
}

func NewAuthMiddleware(secret, issuer string) *AuthMiddleware {
	return &AuthMiddleware{
		secret: []byte(secret),
		issuer: issuer,
	}
}

func (a *AuthMiddleware) Middleware(
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		const prefix = "Bearer "

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, prefix) {
			writeUnauthorized(w)
			return
		}

		tokenString := strings.TrimSpace(
			strings.TrimPrefix(authHeader, prefix),
		)
		if tokenString == "" {
			writeUnauthorized(w)
			return
		}

		claims := &jwt.RegisteredClaims{}

		token, err := jwt.ParseWithClaims(
			tokenString,
			claims,
			func(token *jwt.Token) (any, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, jwt.ErrSignatureInvalid
				}
				return a.secret, nil
			},
			jwt.WithValidMethods([]string{"HS256"}),
			jwt.WithIssuer(a.issuer),
			jwt.WithExpirationRequired(),
			jwt.WithLeeway(30*time.Second),
		)
		if err != nil || token == nil || !token.Valid {
			writeUnauthorized(w)
			return
		}

		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			writeUnauthorized(w)
			return
		}

		ctx := context.WithValue(
			r.Context(),
			userIDKey,
			userID.String(),
		)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	return userID, ok
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "unauthorized",
	})
}
