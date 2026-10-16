package config

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// OpenPostgres opens a tuned pool for dsn and verifies it with a ping.
// name identifies the pool (e.g. "app", "ledger") in errors.
func OpenPostgres(dsn, name string, cfg Config) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s pool: %w", name, err)
	}
	db.SetMaxOpenConns(cfg.PGMaxOpen)
	db.SetMaxIdleConns(cfg.PGMaxIdle)
	db.SetConnMaxLifetime(time.Duration(cfg.PGMaxLifetime) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(cfg.PGMaxIdleTime) * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.PGConnectTimeout)*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping %s: %w", name, err)
	}
	return db, nil
}
