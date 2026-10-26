package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"paymentgateway/internal/config"
	"paymentgateway/internal/payment/relay"
	"paymentgateway/internal/payment/repositories"
	"paymentgateway/pkg/logging"
)

func main() {
	cfg, err := config.Load(".env")
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	appDB, err := config.OpenPostgres("app", cfg.DB)
	if err != nil {
		log.Fatalf("could not open app db: %v", err)
	}
	defer appDB.FromMaster.Close()

	broker, err := config.OpenBroker(cfg)
	if err != nil {
		log.Fatalf("could not open broker: %v", err)
	}
	defer broker.Close()

	outbox := repositories.NewPaymentOutbox(appDB)
	w := relay.NewWorker(outbox, broker, cfg.RabbitMQ.MaxAttempts, 5*time.Second)
	w.Logger = logging.New(os.Stdout)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := w.Run(ctx); err != nil {
		log.Fatalf("outbox relay worker stopped: %v", err)
	}
}
