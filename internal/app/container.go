package app

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	"paymentgateway/internal/config"
	"paymentgateway/internal/merchant"
	"paymentgateway/internal/payment"
	"paymentgateway/pkg/logging"
	"paymentgateway/pkg/tracing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Container struct {
	AppDB    *sql.DB
	LedgerDB *sql.DB
	Redis    *redis.Client
	Logger   *slog.Logger
	Tracer   *sdktrace.TracerProvider
	Config   config.Config
	Merchant *merchant.Handler
	Payment  *payment.Handler
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

	return &Container{AppDB: appDB, LedgerDB: ledgerDB, Redis: rdb, Logger: logger, Tracer: tp, Config: cfg, Merchant: mH, Payment: pH}, nil
}
