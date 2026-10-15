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
	"paymentgateway/internal/merchant"
	"paymentgateway/internal/payment"
	"paymentgateway/internal/payment/models"
	"paymentgateway/internal/payment/repositories"
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
	Broker   *Broker
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
	network, err := cardNetwork(cfg)
	if err != nil {
		return nil, err
	}

	appDB, err := OpenPostgres(cfg.AppDSN, "app", cfg)
	if err != nil {
		return nil, err
	}

	ledgerDB, err := OpenPostgres(cfg.LedgerDSN, "ledger", cfg)
	if err != nil {
		appDB.Close()
		return nil, err
	}

	rdb, err := OpenRedis(cfg)
	if err != nil {
		appDB.Close()
		ledgerDB.Close()
		return nil, err
	}

	broker, err := OpenBroker(cfg)
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
	pLedger := repositories.NewAuditRepository(ledgerDB)
	pSvc := payment.NewService(pApp, pLedger, mRepo, rdb, network, NewWebhookPublisher(broker))
	pH := payment.NewHandler(pSvc)

	return &Container{AppDB: appDB, LedgerDB: ledgerDB, Redis: rdb, Broker: broker, Logger: logger, Tracer: tp, Config: cfg, Merchant: mH, Payment: pH}, nil
}

// cardNetwork builds the card-network client when configured.
// Empty base URL keeps the nil network (Create fails closed); a base
// URL without key is a boot error.
func cardNetwork(cfg config.Config) (models.CardNetwork, error) {
	return repositories.NewMasterCardRepository(
		apiclient.New(nil),
		repositories.MasterCardConfig{
			BaseURL:        cfg.MCBaseURL,
			APIKey:         cfg.MCAPIKey,
			Timeout:        time.Duration(cfg.MCTimeout) * time.Second,
			OverallTimeout: time.Duration(cfg.MCOverallTimeout) * time.Second,
			MaxAttempts:    cfg.MCMaxAttempts,
			BaseBackoff:    time.Duration(cfg.MCBaseBackoff) * time.Millisecond,
			MaxBackoff:     time.Duration(cfg.MCMaxBackoff) * time.Millisecond,
		},
	), nil
}
