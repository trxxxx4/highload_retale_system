package repository

import "errors"

var (
	ErrIdempotencyConflict      = errors.New("idempotency conflict")
	ErrOutOfStock               = errors.New("out of stock")
	ErrProductNotFound          = errors.New("product with current sku not found")
	ErrReservationAlreadyExists = errors.New("reservation is already existing")

	ErrReservationNotExists = errors.New("reservation not exists")
	ErrPaymentConflict      = errors.New("paymentID doesnt match")
	ErrInvalidPaymentState  = errors.New("status of payment is expired or cancelled or time of payment is expired")
	ErrAmountDismatch       = errors.New("amount of payment dismatches with expected amount")

	ErrInvalidCancelState = errors.New("reservation cannot be cancelled")

	ErrLimitIsNotAllowed = errors.New("limit more than 100 or less than 1")
)
