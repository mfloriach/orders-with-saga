package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"paymentgateway/internal/app"
	"paymentgateway/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	c, err := app.Build(cfg)
	if err != nil {
		log.Fatal(err)
	}

	app, err := app.NewServer(*c)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.Shutdown(ctx)
		_ = app.Shutdown()
	}()
	log.Fatal(app.Listen(fmt.Sprintf(":%d", cfg.Port)))
}
