package repository

import (
	"context"
	"db/internal/models"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReservationRepository struct {
	Pool *pgxpool.Pool
}

func NewReservationRepository(
	pool *pgxpool.Pool,
) *ReservationRepository {
	return &ReservationRepository{
		Pool: pool,
	}
}
func (repo *ReservationRepository) GetByID(
	ctx context.Context,
	id string,
) (
	*models.Reservation,
	error,
) {
	query := `SELECT 
		id,
		user_id,
		product_id,
		qty,
		status,
		expires_at,
		idempotency_key,
		created_at
	FROM reservations
	WHERE id = $1;
	`
	var reservation models.Reservation
	err := repo.Pool.QueryRow(ctx, query, id).Scan(
		&reservation.ID,
		&reservation.UserID,
		&reservation.ProductID,
		&reservation.Qty,
		&reservation.Status,
		&reservation.ExpiresAt,
		&reservation.IdempotencyKey,
		&reservation.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrReservationNotExists
	}
	if err != nil {
		return nil, err
	}

	return &reservation, nil
}

func (repo *ReservationRepository) Create(
	ctx context.Context,
	dto models.CreateReservationDTO,

) (
	*models.ReservationResponse,
	error,
) {

	tx, err := repo.Pool.Begin(ctx)

	if err != nil {
		return nil, err

	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	getIDAndPriceQuery := `
	SELECT 
		id,
		price_cents
	FROM products
	WHERE sku = $1;`
	var product_id_and_priceDTO = struct {
		ID         int
		PriceCents int
	}{}

	err = tx.QueryRow(
		ctx,
		getIDAndPriceQuery,
		dto.SKU,
	).Scan(
		&product_id_and_priceDTO.ID,
		&product_id_and_priceDTO.PriceCents,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProductNotFound
	}
	if err != nil {
		return nil, err
	}
	now := time.Now()
	expiresAt := now.Add(time.Duration(dto.TTLSeconds) * time.Second)

	var reservation models.Reservation
	err = tx.QueryRow(ctx, `
    INSERT INTO reservations (
        id, user_id, product_id, qty, status,
        expires_at, idempotency_key, created_at
    )
    VALUES ($1, $2, $3, $4, 'active', $5, $6, $7)
    ON CONFLICT (user_id, idempotency_key) DO NOTHING
    RETURNING id, user_id, product_id, qty, status,
              expires_at, idempotency_key, created_at`,
		dto.ID, dto.UserID, product_id_and_priceDTO.ID, dto.Qty,
		expiresAt, dto.IdempotencyKey, now,
	).Scan(
		&reservation.ID, &reservation.UserID, &reservation.ProductID,
		&reservation.Qty, &reservation.Status, &reservation.ExpiresAt,
		&reservation.IdempotencyKey, &reservation.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		// бронь с таким ключом уже есть, склад не трогаем
		err = tx.QueryRow(ctx, `
        SELECT id, user_id, product_id, qty, status,
               expires_at, idempotency_key, created_at
        FROM reservations
        WHERE user_id = $1 AND idempotency_key = $2`,
			dto.UserID, dto.IdempotencyKey,
		).Scan(
			&reservation.ID, &reservation.UserID, &reservation.ProductID,
			&reservation.Qty, &reservation.Status, &reservation.ExpiresAt,
			&reservation.IdempotencyKey, &reservation.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		if reservation.ProductID != product_id_and_priceDTO.ID ||
			reservation.Qty != dto.Qty {
			return nil, ErrIdempotencyConflict
		}
		return &models.ReservationResponse{
			ID:          reservation.ID,
			SKU:         dto.SKU,
			Qty:         reservation.Qty,
			Status:      reservation.Status,
			ExpiresAt:   reservation.ExpiresAt,
			AmountCents: product_id_and_priceDTO.PriceCents * reservation.Qty,
		}, ErrReservationAlreadyExists
	}
	if err != nil {
		return nil, err
	}

	// бронь новая: резервируем склад
	result, err := tx.Exec(ctx, `
    UPDATE stock
    SET reserved = reserved + $1
    WHERE product_id = $2
      AND on_hand - reserved >= $1`,
		dto.Qty, product_id_and_priceDTO.ID,
	)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() == 0 {
		return nil, ErrOutOfStock // откат отменит и вставку
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	response := models.ReservationResponse{
		ID:          reservation.ID,
		SKU:         dto.SKU,
		Qty:         reservation.Qty,
		Status:      reservation.Status,
		ExpiresAt:   reservation.ExpiresAt,
		AmountCents: product_id_and_priceDTO.PriceCents * reservation.Qty,
	}
	return &response, nil

}

func (repo *ReservationRepository) Pay(
	ctx context.Context,
	reservationID string,
	dto models.PayReservationDTO,
) (*models.Payment, error) {
	tx, err := repo.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var reservation struct {
		ID        string
		ProductID int
		Qty       int
		Status    string
		ExpiresAt time.Time
	}

	selectReservationQuery := `
		SELECT id, product_id, qty, status, expires_at
		FROM reservations
		WHERE id = $1
		FOR UPDATE;
	`

	err = tx.QueryRow(
		ctx,
		selectReservationQuery,
		reservationID,
	).Scan(
		&reservation.ID,
		&reservation.ProductID,
		&reservation.Qty,
		&reservation.Status,
		&reservation.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrReservationNotExists
	}
	if err != nil {
		return nil, err
	}

	var payment models.Payment

	if reservation.Status == "paid" {
		err = tx.QueryRow(
			ctx,
			`SELECT payment_id, reservation_id,
			        amount_cents, created_at
			 FROM payments
			 WHERE reservation_id = $1;`,
			reservation.ID,
		).Scan(
			&payment.PaymentID,
			&payment.ReservationID,
			&payment.AmountCents,
			&payment.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if payment.PaymentID != dto.PaymentID {
			return nil, ErrPaymentConflict
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}

		return &payment, nil
	}

	if reservation.Status != "active" {
		return nil, ErrInvalidPaymentState
	}

	var nowOnDatabase time.Time

	err = tx.QueryRow(ctx, "SELECT now();").Scan(
		&nowOnDatabase,
	)
	if err != nil {
		return nil, err
	}

	if !nowOnDatabase.Before(reservation.ExpiresAt) {
		return nil, ErrInvalidPaymentState
	}

	var priceCents int

	err = tx.QueryRow(
		ctx,
		`SELECT price_cents
		 FROM products
		 WHERE id = $1;`,
		reservation.ProductID,
	).Scan(&priceCents)
	if err != nil {
		return nil, err
	}

	expectedAmount := priceCents * reservation.Qty

	if expectedAmount != dto.AmountCents {
		return nil, ErrAmountDismatch
	}

	insertPaymentQuery := `
		INSERT INTO payments (
			payment_id,
			reservation_id,
			amount_cents
		)
		VALUES ($1, $2, $3)
		RETURNING
			payment_id,
			reservation_id,
			amount_cents,
			created_at;
	`

	err = tx.QueryRow(
		ctx,
		insertPaymentQuery,
		dto.PaymentID,
		reservation.ID,
		dto.AmountCents,
	).Scan(
		&payment.PaymentID,
		&payment.ReservationID,
		&payment.AmountCents,
		&payment.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	result, err := tx.Exec(
		ctx,
		`UPDATE reservations
		 SET status = 'paid'
		 WHERE id = $1;`,
		reservation.ID,
	)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() != 1 {
		return nil, errors.New("failed to update reservation")
	}

	result, err = tx.Exec(
		ctx,
		`UPDATE stock
		 SET
			on_hand = on_hand - $1,
			reserved = reserved - $1
		 WHERE product_id = $2;`,
		reservation.Qty,
		reservation.ProductID,
	)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() != 1 {
		return nil, errors.New("failed to update stock")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &payment, nil
}

func (repo *ReservationRepository) Cancel(
	ctx context.Context,
	reservationID string,
	dto models.CancelReservationDTO,
) (*models.CancelReservationResponse, error) {
	tx, err := repo.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var reservation struct {
		ID        string
		ProductID int
		Qty       int
		Status    string
		SKU       string
	}

	query := `
        SELECT r.id, r.product_id, r.qty, r.status, p.sku
        FROM reservations AS r
        JOIN products AS p ON p.id = r.product_id
        WHERE r.id = $1 AND r.user_id = $2
        FOR UPDATE OF r;
    `

	err = tx.QueryRow(
		ctx,
		query,
		reservationID,
		dto.UserID,
	).Scan(
		&reservation.ID,
		&reservation.ProductID,
		&reservation.Qty,
		&reservation.Status,
		&reservation.SKU,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrReservationNotExists
	}
	if err != nil {
		return nil, err
	}

	response := &models.CancelReservationResponse{
		SKU: reservation.SKU,
		Qty: reservation.Qty,
	}

	if reservation.Status == "cancelled" {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return response, nil
	}

	if reservation.Status != "active" {
		return nil, ErrInvalidCancelState
	}

	_, err = tx.Exec(
		ctx,
		`UPDATE reservations
         SET status = 'cancelled'
         WHERE id = $1;`,
		reservation.ID,
	)
	if err != nil {
		return nil, err
	}

	result, err := tx.Exec(
		ctx,
		`UPDATE stock
         SET reserved = reserved - $1
         WHERE product_id = $2
           AND reserved >= $1;`,
		reservation.Qty,
		reservation.ProductID,
	)
	if err != nil {
		return nil, err
	}

	if result.RowsAffected() != 1 {
		return nil, errors.New("failed to update stock")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return response, nil
}

func (repo *ReservationRepository) Expire(
	ctx context.Context,
	limit *int,
) ([]models.ExpiredReservaionResponse, error) {
	if limit == nil {
		defaultLimit := 500
		limit = &defaultLimit
	}

	if *limit < 1 || *limit > 1000 {
		return nil, ErrLimitIsNotAllowed
	}

	tx, err := repo.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	result := make([]models.ExpiredReservaionResponse, 0)
	uuidSlice := make([]string, 0)

	rows, err := tx.Query(ctx, `
        SELECT r.id, p.sku, r.qty, r.product_id
		FROM reservations AS r
		JOIN products AS p ON p.id = r.product_id
		WHERE r.status = 'active'
			AND r.expires_at <= now()
		ORDER BY r.expires_at
		LIMIT $1
		FOR UPDATE OF r SKIP LOCKED;
    `, *limit)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var reservation models.ExpiredReservaionResponse

		if err := rows.Scan(
			&reservation.ID,
			&reservation.SKU,
			&reservation.Qty,
			&reservation.ProductID,
		); err != nil {
			rows.Close()
			return nil, err
		}

		result = append(result, reservation)
		uuidSlice = append(uuidSlice, reservation.ID)
	}

	rowsErr := rows.Err()
	rows.Close()

	if rowsErr != nil {
		return nil, rowsErr
	}

	if len(uuidSlice) > 0 {
		updateResult, err := tx.Exec(ctx, `
            UPDATE reservations
            SET status = 'expired'
            WHERE id = ANY($1::uuid[]);
        `, uuidSlice)
		if err != nil {
			return nil, err
		}

		if updateResult.RowsAffected() != int64(len(uuidSlice)) {
			return nil, errors.New(
				"failed to expire reservations",
			)
		}

		for _, reservation := range result {
			stockResult, err := tx.Exec(ctx, `
                UPDATE stock
                SET reserved = reserved - $1
                WHERE product_id = $2
                  AND reserved >= $1;
            `,
				reservation.Qty,
				reservation.ProductID,
			)
			if err != nil {
				return nil, err
			}

			if stockResult.RowsAffected() != 1 {
				return nil, errors.New(
					"failed to release reserved stock",
				)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return result, nil
}
