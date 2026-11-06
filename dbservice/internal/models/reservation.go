package models

import "time"

type Reservation struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	ProductID      int       `json:"product_id"`
	Qty            int       `json:"qty"`
	Status         string    `json:"status"`
	ExpiresAt      time.Time `json:"expires_at"`
	IdempotencyKey string    `json:"idempotency_key"`
	CreatedAt      time.Time `json:"created_at"`
}
type ReservationResponse struct {
	ID          string    `json:"id"`
	SKU         string    `json:"sku"`
	Qty         int       `json:"qty"`
	Status      string    `json:"status"`
	ExpiresAt   time.Time `json:"expires_at"`
	AmountCents int       `json:"amount_cents"`
}

type CreateReservationDTO struct {
	ID             string `json:"id"`
	UserID         string `json:"user_id"`
	SKU            string `json:"sku"`
	Qty            int    `json:"qty"`
	IdempotencyKey string `json:"idempotency_key"`
	TTLSeconds     int    `json:"ttl_seconds"`
}

type Payment struct {
	PaymentID     string    `json:"payment_id"`
	ReservationID string    `json:"reservation_id"`
	AmountCents   int       `json:"amount_cents"`
	CreatedAt     time.Time `json:"created_at"`
}

type PayReservationDTO struct {
	PaymentID   string `json:"payment_id"`
	AmountCents int    `json:"amount_cents"`
}

type CancelReservationDTO struct {
	UserID string `json:"user_id"`
}

type CancelReservationResponse struct {
	SKU string `json:"sku"`
	Qty int    `json:"qty"`
}

type ExpiredReservaionResponse struct {
	ID        string `json:"id"`
	ProductID int    `json:"product_id"`
	Qty       int    `json:"qty"`
	SKU       string `json:"sku"`
}

type RegisterResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}
