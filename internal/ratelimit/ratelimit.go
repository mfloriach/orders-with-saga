package ratelimit

import (
	"context"
	"fmt"
	"time"

	"paymentgateway/pkg/apperr"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// ponytail: fixed-window INCR+EXPIRE, sliding window if abuse patterns emerge
func Middleware(rdb *redis.Client, limit int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := fmt.Sprintf("rl:%s", c.IP())
		ctx := context.Background()
		n, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			return c.Next() // fail open if redis down
		}
		if n == 1 {
			rdb.Expire(ctx, key, window)
		}
		if n > int64(limit) {
			ttl, _ := rdb.TTL(ctx, key).Result()
			return apperr.RateLimitedError(int(ttl.Seconds()) + 1)
		}
		return c.Next()
	}
}
