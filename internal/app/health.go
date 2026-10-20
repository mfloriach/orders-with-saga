package app

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Health is the dumb liveness probe; its contract never changes.
func Health(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) }

// readyTimeout bounds the whole readiness sweep.
const readyTimeout = 2 * time.Second

// readyHandler builds a readiness probe over named checks: 200 when all
// pass, 503 naming the failures. Checks run sequentially under one budget.
func readyHandler(checks map[string]func(ctx context.Context) error) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), readyTimeout)
		defer cancel()

		deps := make(fiber.Map, len(checks))
		var failed []string
		for name, check := range checks {
			if err := check(ctx); err != nil {
				deps[name] = "down"
				failed = append(failed, name)
				continue
			}
			deps[name] = "ok"
		}
		if len(failed) > 0 {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"ok": false, "deps": deps, "failed": failed,
			})
		}
		return c.JSON(fiber.Map{"ok": true, "deps": deps})
	}
}

// readyChecks wires the container's pools into named checks.
func readyChecks(cnt *Container) map[string]func(ctx context.Context) error {
	return map[string]func(ctx context.Context) error{
		"app_db":    cnt.AppDB.PingContext,
		"ledger_db": cnt.LedgerDB.PingContext,
		"redis":     func(ctx context.Context) error { return cnt.Redis.Ping(ctx).Err() },
	}
}
