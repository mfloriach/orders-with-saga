package config

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Port       int    `env:"PORT" env-default:"8080" validate:"required,min=1,max=65535"`
	AppDSN     string `env:"APP_DSN" env-required:"true" validate:"required"`
	LedgerDSN  string `env:"LEDGER_DSN" env-required:"true" validate:"required"`
	RedisAddr  string `env:"REDIS_ADDR" env-required:"true" validate:"required"`
	RateLimit  int    `env:"RATE_LIMIT" env-default:"100" validate:"gt=0"`
	RateWindow int    `env:"RATE_WINDOW_SEC" env-default:"60" validate:"gt=0"`
}

func Load() (Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return Config{}, err
	}
	if err := validator.New().Struct(cfg); err != nil {
		if ve, ok := err.(validator.ValidationErrors); ok {
			fe := ve[0]
			return Config{}, fmt.Errorf("invalid %s: failed %q validation", fe.Field(), fe.Tag())
		}
		return Config{}, err
	}
	return cfg, nil
}
