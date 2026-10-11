package app

import (
	"time"

	"paymentgateway/internal/ratelimit"
	"paymentgateway/pkg/apperr"
	"paymentgateway/pkg/logging"

	"github.com/gofiber/contrib/otelfiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

func NewServer(cnt Container) (*fiber.App, error) {
	f := fiber.New(fiber.Config{ErrorHandler: apperr.Handler})

	configure(f, cnt)
	routes(f, cnt)

	return f, nil
}

func configure(f *fiber.App, cnt Container) {
	f.Use(requestid.New())
	f.Use(logging.Bridge())
	f.Use(otelfiber.Middleware(otelfiber.WithTracerProvider(cnt.Tracer)))
	f.Use(logging.Stash())
	f.Use(logging.RequestLogger(cnt.Logger))
}

func routes(f *fiber.App, cnt Container) {
	f.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	f.Use(ratelimit.Middleware(cnt.Redis, cnt.Config.RateLimit, time.Duration(cnt.Config.RateWindow)*time.Second))
	f.Post("/merchants", cnt.Merchant.Create)
	f.Post("/payments", cnt.Payment.Create)
}
