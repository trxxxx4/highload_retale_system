package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewClient(addr, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func NewClientWithPing(
	ctx context.Context,
	addr, password string,
	db int,
) (*redis.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	client := NewClient(addr, password, db)

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return client, nil
}
