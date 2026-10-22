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
	f.Get("/health", Health)

	f.Get("/ready", readyHandler(readyChecks(&cnt)))

	f.Get("/openapi.yaml", openAPIHandler)

	f.Get("/docs", docsHandler)

	f.Use(ratelimit.Middleware(cnt.Redis, cnt.Config.RateLimit, time.Duration(cnt.Config.RateWindow)*time.Second))
	f.Post("/merchants", cnt.Merchant.Create)
	f.Get("/merchants", cnt.Merchant.List)
	f.Get("/merchants/:id", cnt.Merchant.Get)
	f.Patch("/merchants/:id", cnt.Merchant.Update)
	f.Delete("/merchants/:id", cnt.Merchant.Delete)

	f.Post("/payments", cnt.Payment.Create)
}
