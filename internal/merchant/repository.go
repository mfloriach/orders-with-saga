package merchant

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrEmailTaken = errors.New("email_taken")
var ErrNotFound = errors.New("merchant_not_found")

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db} }

func (r *Repository) Create(ctx context.Context, m *Merchant) error {
	m.ID = uuid.NewString()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO merchants (id, name, email, webhook_url) VALUES ($1,$2,$3,$4)`,
		m.ID, m.Name, m.Email, m.WebhookURL)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEmailTaken
		}
		return err
	}
	return r.db.QueryRowContext(ctx,
		`SELECT created_at FROM merchants WHERE id=$1`, m.ID).Scan(&m.CreatedAt)
}

func (r *Repository) FindByID(ctx context.Context, id string) (*Merchant, error) {
	var m Merchant
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, webhook_url, created_at FROM merchants WHERE id=$1`, id).
		Scan(&m.ID, &m.Name, &m.Email, &m.WebhookURL, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &m, err
}
