package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"paymentgateway/internal/config"
	"paymentgateway/internal/domain"
	"paymentgateway/internal/merchant"
	"paymentgateway/internal/payment"
	"paymentgateway/internal/payment/repositories"
	"paymentgateway/internal/payment/workflow"
	"paymentgateway/pkg/apiclient"
	"paymentgateway/pkg/logging"
	"paymentgateway/pkg/tracing"

	"github.com/redis/go-redis/v9"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Container struct {
	AppDB    *sql.DB
	LedgerDB *sql.DB
	Redis    *redis.Client
	Broker   *config.Broker
	Logger   *slog.Logger
	Tracer   *sdktrace.TracerProvider
	Config   config.Config
	Merchant *merchant.Handler
	Payment  *payment.Handler

	closed bool
}

// Shutdown closes pools and flushes telemetry. Every step runs even if an
// earlier one fails; errors join with context. Idempotent: repeats return nil.
func (c *Container) Shutdown(ctx context.Context) error {
	if c.closed {
		return nil
	}
	c.closed = true
	var errs []error
	if err := c.Broker.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close rabbitmq: %w", err))
	}
	if err := c.Redis.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close redis: %w", err))
	}
	if err := c.AppDB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close app db: %w", err))
	}
	if err := c.LedgerDB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close ledger db: %w", err))
	}
	if err := c.Tracer.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("shutdown tracer: %w", err))
	}
	return errors.Join(errs...)
}

func Build(cfg config.Config) (*Container, error) {
	network, err := NewCardNetwork(cfg)
	if err != nil {
		return nil, err
	}

	appDB, err := config.OpenPostgres(cfg.AppDSN, "app", cfg)
	if err != nil {
		return nil, err
	}

	ledgerDB, err := config.OpenPostgres(cfg.LedgerDSN, "ledger", cfg)
	if err != nil {
		appDB.Close()
		return nil, err
	}

	rdb, err := config.OpenRedis(cfg)
	if err != nil {
		appDB.Close()
		ledgerDB.Close()
		return nil, err
	}

	broker, err := config.OpenBroker(cfg)
	if err != nil {
		appDB.Close()
		ledgerDB.Close()
		rdb.Close()
		return nil, err
	}

	logger := logging.New(os.Stdout)
	slog.SetDefault(logger)
	tp := tracing.Init()

	mRepo := merchant.NewRepository(appDB)
	mSvc := merchant.NewService(mRepo)
	mH := merchant.NewHandler(mSvc)

	pApp := repositories.NewPaymentDatabaseRepository(appDB)
	pSvc := payment.NewService(pApp, mRepo, workflow.NewWorkflow(pApp, network))
	pH := payment.NewHandler(pSvc)

	return &Container{AppDB: appDB, LedgerDB: ledgerDB, Redis: rdb, Broker: broker, Logger: logger, Tracer: tp, Config: cfg, Merchant: mH, Payment: pH}, nil
}

// NewCardNetwork builds the card-network client from config and wraps it in the
// repository. The client owns the base URL, auth header, per-attempt timeout,
// and retry policy — including the repo's Retryable classification. An empty
// base URL leaves a client whose requests all fail, so payment creation fails
// closed at the network step.
func NewCardNetwork(cfg config.Config) (domain.CardNetwork, error) {
	opts := []apiclient.Option{
		apiclient.WithBaseURL(cfg.MCBaseURL),
		apiclient.WithHeader("Content-Type", "application/json"),
		apiclient.WithTimeout(time.Duration(cfg.MCTimeout) * time.Second),
		apiclient.WithRetry(
			cfg.MCMaxAttempts,
			time.Duration(cfg.MCBaseBackoff)*time.Millisecond,
			time.Duration(cfg.MCMaxBackoff)*time.Millisecond,
			nil,
		),
	}
	if cfg.MCAPIKey != "" {
		opts = append(opts, apiclient.WithHeader("Authorization", "Bearer "+cfg.MCAPIKey))
	}

	return repositories.NewMasterCardRepository(apiclient.New(nil, opts...)), nil
}
