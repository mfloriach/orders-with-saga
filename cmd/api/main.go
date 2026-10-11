package main

import (
	"log"

	"paymentgateway/internal/app"
	"paymentgateway/internal/config"
)

func main() {
	cfg := config.Load()
	c, err := app.Build(cfg)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(c.App.Listen(":" + cfg.Port))
}
