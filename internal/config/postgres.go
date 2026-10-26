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
func OpenPostgres(name string, cfg Postgress) (*Database, error) {
	db, err := sql.Open("pgx", cfg.DSN)
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

	return &Database{FromMaster: db}, nil
}

type Database struct {
	FromMaster *sql.DB
}

type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func Executor(ctx context.Context, db *sql.DB) DBTX {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return db
}

type txKey struct{}

func WithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func TxFromContext(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*sql.Tx)
	return tx, ok
}

func WithinTx(
	ctx context.Context,
	db *sql.DB,
	fn func(context.Context) error,
) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	txCtx := WithTx(ctx, tx)

	if err := fn(txCtx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
