package config

import (
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
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
	return load(".env")
}

// load reads path when it exists and falls back to OS env alone, so
// containers without a file keep working. OS env always wins over the
// file (godotenv never overrides existing vars; cleanenv.ReadConfig would).
func load(path string) (Config, error) {
	var cfg Config
	if _, statErr := os.Stat(path); statErr == nil {
		if err := godotenv.Load(path); err != nil {
			return Config{}, err
		}
	}

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
