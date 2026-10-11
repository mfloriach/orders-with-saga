package app

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"time"

	"paymentgateway/internal/config"
	"paymentgateway/internal/merchant"
	"paymentgateway/internal/payment"
	"paymentgateway/internal/ratelimit"
	"paymentgateway/pkg/apperr"
	"paymentgateway/pkg/logging"
	"paymentgateway/pkg/tracing"

	"github.com/gofiber/contrib/otelfiber"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Container struct {
	App      *fiber.App
	AppDB    *sql.DB
	LedgerDB *sql.DB
	Redis    *redis.Client
	Logger   *slog.Logger
	Tracer   *sdktrace.TracerProvider
}

// Shutdown flushes telemetry (no-op until an exporter is configured).
func (c *Container) Shutdown(ctx context.Context) error {
	return c.Tracer.Shutdown(ctx)
}

func Build(cfg config.Config) (*Container, error) {
	appDB, err := sql.Open("pgx", cfg.AppDSN)
	if err != nil {
		return nil, err
	}
	ledgerDB, err := sql.Open("pgx", cfg.LedgerDSN)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	logger := logging.New(os.Stdout)
	slog.SetDefault(logger)
	tp := tracing.Init()

	mRepo := merchant.NewRepository(appDB)
	mSvc := merchant.NewService(mRepo)
	mH := merchant.NewHandler(mSvc)

	pApp := payment.NewAppRepository(appDB)
	pLedger := payment.NewLedgerRepository(ledgerDB)
	pSvc := payment.NewService(pApp, pLedger, mRepo, rdb)
	pH := payment.NewHandler(pSvc)

	f := fiber.New(fiber.Config{ErrorHandler: apperr.Handler})
	f.Use(requestid.New())
	f.Use(logging.Bridge())
	f.Use(otelfiber.Middleware(otelfiber.WithTracerProvider(tp)))
	f.Use(logging.Stash())
	f.Use(logging.RequestLogger(logger))
	f.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	f.Use(ratelimit.Middleware(rdb, cfg.RateLimit, time.Duration(cfg.RateWindow)*time.Second))
	f.Post("/merchants", mH.Create)
	f.Post("/payments", pH.Create)

	return &Container{App: f, AppDB: appDB, LedgerDB: ledgerDB, Redis: rdb, Logger: logger, Tracer: tp}, nil
}
