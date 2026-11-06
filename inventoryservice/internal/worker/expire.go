package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"inventory/internal/http/dbclient"
	redisstore "inventory/internal/redis"

	goredis "github.com/redis/go-redis/v9"
)

const stockPageLimit = 5000

type ExpiryWorker struct {
	DBClient *dbclient.Client
	Redis    *goredis.Client
	Interval time.Duration
	Limit    int
}

func NewExpiryWorker(
	dbClient *dbclient.Client,
	redisClient *goredis.Client,
	interval time.Duration,
	limit int,
) *ExpiryWorker {
	return &ExpiryWorker{
		DBClient: dbClient,
		Redis:    redisClient,
		Interval: interval,
		Limit:    limit,
	}
}

func (w *ExpiryWorker) Run(ctx context.Context) {
	if w.Interval <= 0 {
		log.Print("expiry worker: interval must be positive")
		return
	}

	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:

			if err := w.ProcessOnce(ctx); err != nil {
				log.Printf("expiry worker: %v", err)
			}
		}
	}
}

func (w *ExpiryWorker) ProcessOnce(
	ctx context.Context,
) error {
	var firstErr error

	// PostgreSQL обрабатывает истёкшие бронирования
	// и освобождает остатки.
	_, err := w.DBClient.Expire(ctx, w.Limit)
	if err != nil {
		firstErr = fmt.Errorf(
			"expire reservations: %w",
			err,
		)
	}

	// Даже если обработка истёкших бронирований завершилась
	// ошибкой, пробуем восстановить кэш из PostgreSQL.
	if err := w.RefreshStockCache(ctx); err != nil {
		if firstErr == nil {
			firstErr = fmt.Errorf(
				"refresh stock cache: %w",
				err,
			)
		} else {
			log.Printf(
				"expiry worker: stock refresh also failed: %v",
				err,
			)
		}
	}

	return firstErr
}

// RefreshStockCache последовательно загружает остатки
// из PostgreSQL через db-service и записывает их в Redis.
func (w *ExpiryWorker) RefreshStockCache(
	ctx context.Context,
) error {
	after := ""

	for {
		page, err := w.DBClient.Stocks(
			ctx,
			after,
			stockPageLimit,
		)
		if err != nil {
			return fmt.Errorf(
				"get stock page after %q: %w",
				after,
				err,
			)
		}

		if len(page.Items) == 0 {
			return nil
		}

		for _, item := range page.Items {
			if err := redisstore.SetStock(
				ctx,
				w.Redis,
				item.SKU,
				item.Available,
			); err != nil {
				return fmt.Errorf(
					"update cached stock for %s: %w",
					item.SKU,
					err,
				)
			}
		}

		if page.NextAfter == "" {
			return nil
		}

		if page.NextAfter == after {
			return fmt.Errorf(
				"stock pagination did not advance after %q",
				after,
			)
		}

		after = page.NextAfter
	}
}
