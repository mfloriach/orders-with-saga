package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"paymentgateway/internal/app"
	"paymentgateway/internal/config"
	"paymentgateway/internal/merchant"
	"paymentgateway/internal/webhooks"
	"paymentgateway/pkg/logging"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	appDB, err := app.OpenPostgres(cfg.AppDSN, "app", cfg)
	if err != nil {
		log.Fatalf("could not open app db: %v", err)
	}
	defer appDB.Close()

	broker, err := app.OpenBroker(cfg)
	if err != nil {
		log.Fatalf("could not open broker: %v", err)
	}
	defer broker.Close()

	ch, err := broker.Channel()
	if err != nil {
		log.Fatalf("could not open channel: %v", err)
	}
	defer ch.Close()

	logger := logging.New(os.Stdout)
	consumer := webhooks.NewConsumer(ch,
		merchant.NewRepository(appDB),
		cfg.AMQPMaxAttempts,
		time.Duration(cfg.AMQPRetryTTL)*time.Millisecond,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := consumer.Run(ctx); err != nil {
		logger.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
}
