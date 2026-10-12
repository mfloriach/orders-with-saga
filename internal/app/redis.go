package app

import (
	"context"
	"fmt"
	"time"

	"paymentgateway/internal/config"

	"github.com/redis/go-redis/v9"
)

// OpenRedis opens a tuned client and verifies it with a ping.
func OpenRedis(cfg config.Config) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		PoolSize:     cfg.RedisPoolSize,
		DialTimeout:  time.Duration(cfg.RedisDialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.RedisReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.RedisWriteTimeout) * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.RedisDialTimeout)*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return rdb, nil
}
