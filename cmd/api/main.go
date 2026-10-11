package main

import (
	"fmt"
	"log"

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
	log.Fatal(c.App.Listen(fmt.Sprintf(":%d", cfg.Port)))
}
