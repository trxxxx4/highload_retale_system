package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
	secret    []byte
	ttl       time.Duration
	jwtIssuer string
}

func NewJWTManager(
	secret string,
	ttl time.Duration,
	jwtIssuer string,
) (*JWTManager, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT secret must be at least 32 bytes")
	}

	if ttl <= 0 {
		return nil, errors.New("JWT TTL must be positive")
	}

	return &JWTManager{
		secret:    []byte(secret),
		ttl:       ttl,
		jwtIssuer: jwtIssuer,
	}, nil
}

func (manager *JWTManager) GetTTL() int {
	return int(manager.ttl / time.Second)
}

func (manager *JWTManager) GenerateToken(
	userID string,
) (string, error) {
	now := time.Now()

	claims := jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    manager.jwtIssuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(manager.ttl)),
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	signedToken, err := token.SignedString(manager.secret)
	if err != nil {
		return "", err
	}

	return signedToken, nil
}
