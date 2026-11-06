package redis

import (
	"context"
	"fmt"
	"strconv"

	goredis "github.com/redis/go-redis/v9"
)

// Stock возвращает доступный остаток из Redis.
// Если ключ отсутствует, вернётся goredis.Nil.
func Stock(
	ctx context.Context,
	client *goredis.Client,
	sku string,
) (int, error) {
	value, err := client.Get(
		ctx,
		"stock:"+sku,
	).Result()
	if err != nil {
		return 0, err
	}

	available, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf(
			"parse cached stock for %s: %w",
			sku,
			err,
		)
	}

	if available < 0 {
		return 0, fmt.Errorf(
			"cached stock for %s is negative",
			sku,
		)
	}

	return available, nil
}

// SetStock записывает точное значение остатка в Redis.
// Значение должно быть получено из PostgreSQL.
func SetStock(
	ctx context.Context,
	client *goredis.Client,
	sku string,
	available int,
) error {
	if sku == "" || available < 0 {
		return fmt.Errorf("invalid stock value")
	}

	return client.Set(
		ctx,
		"stock:"+sku,
		available,
		0,
	).Err()
}

// SetStockIfAbsent заполняет кэш, только если ключ отсутствует.
// Существующее значение не перезаписывается.
func SetStockIfAbsent(
	ctx context.Context,
	client *goredis.Client,
	sku string,
	available int,
) error {
	if sku == "" || available < 0 {
		return fmt.Errorf("invalid stock value")
	}

	return client.SetNX(
		ctx,
		"stock:"+sku,
		available,
		0,
	).Err()
}
