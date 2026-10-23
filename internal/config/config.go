package config

import (
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Postgress struct {
	DSN              string `env:"DSN" env-required:"true" validate:"required"`
	PGMaxOpen        int    `env:"MAX_OPEN" env-default:"25" validate:"gt=0"`
	PGMaxIdle        int    `env:"MAX_IDLE" env-default:"5" validate:"gte=0"`
	PGMaxLifetime    int    `env:"MAX_LIFETIME_SEC" env-default:"300" validate:"gt=0"`
	PGMaxIdleTime    int    `env:"MAX_IDLE_SEC" env-default:"60" validate:"gt=0"`
	PGConnectTimeout int    `env:"CONNECT_TIMEOUT_SEC" env-default:"5" validate:"gt=0"`
}

type Redis struct {
	Addr         string `env:"ADDR" env-required:"true" validate:"required"`
	PoolSize     int    `env:"POOL_SIZE" env-default:"0" validate:"gte=0"`
	DialTimeout  int    `env:"DIAL_TIMEOUT_SEC" env-default:"5" validate:"gt=0"`
	ReadTimeout  int    `env:"READ_TIMEOUT_SEC" env-default:"3" validate:"gt=0"`
	WriteTimeout int    `env:"WRITE_TIMEOUT_SEC" env-default:"3" validate:"gt=0"`
}

type Email struct {
	Host string `env:"HOST" env-default:""`
	Port int    `env:"PORT" env-default:"587" validate:"min=1,max=65535"`
	User string `env:"USER" env-default:""`
	Pass string `env:"PASS" env-default:""`
	TLS  string `env:"TLS" env-default:"starttls" validate:"oneof=none starttls tls"`
	From string `env:"FROM" env-default:"noreply@localhost"`
}

type HTTPClient struct {
	BaseURL     string `env:"BASE_URL" env-default:""`
	APIKey      string `env:"API_KEY" env-default:""`
	Timeout     int    `env:"TIMEOUT_SEC" env-default:"5" validate:"gt=0"`
	MaxAttempts int    `env:"MAX_ATTEMPTS" env-default:"3" validate:"gt=0"`
	BaseBackoff int    `env:"BASE_BACKOFF_MS" env-default:"100" validate:"gt=0"`
	MaxBackoff  int    `env:"MAX_BACKOFF_MS" env-default:"2000" validate:"gt=0"`
}

type Config struct {
	Port int `env:"PORT" env-default:"8080" validate:"required,min=1,max=65535"`

	Redis       Redis      `env-prefix:"REDIS_"`
	DB          Postgress  `env-prefix:"DB_"`
	Ledger      Postgress  `env-prefix:"LEDGER_"`
	Email       Email      `env-prefix:"SMTP_"`
	Webhook     HTTPClient `env-prefix:"WH_"`
	CardNetwork HTTPClient `env-prefix:"MC_"`
}

// load reads path when it exists and falls back to OS env alone, so
// containers without a file keep working. OS env always wins over the
// file (godotenv never overrides existing vars; cleanenv.ReadConfig would).
func Load(path string) (Config, error) {
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
