package app

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v2"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"paymentgateway/internal/config"
	"paymentgateway/internal/merchant"
	"paymentgateway/internal/payment"
	"paymentgateway/internal/ratelimit"
	"paymentgateway/pkg/apperr"
)

type Container struct {
	App     *fiber.App
	AppDB   *sql.DB
	LedgerDB *sql.DB
	Redis   *redis.Client
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

	mRepo := merchant.NewRepository(appDB)
	mSvc := merchant.NewService(mRepo)
	mH := merchant.NewHandler(mSvc)

	pApp := payment.NewAppRepository(appDB)
	pLedger := payment.NewLedgerRepository(ledgerDB)
	pSvc := payment.NewService(pApp, pLedger, mRepo, rdb)
	pH := payment.NewHandler(pSvc)

	f := fiber.New(fiber.Config{ErrorHandler: apperr.Handler})
	f.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	f.Use(ratelimit.Middleware(rdb, cfg.RateLimit, time.Duration(cfg.RateWindow)*time.Second))
	f.Post("/merchants", mH.Create)
	f.Post("/payments", pH.Create)

	return &Container{App: f, AppDB: appDB, LedgerDB: ledgerDB, Redis: rdb}, nil
}
