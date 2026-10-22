package merchant

import (
	"context"
	"database/sql"
	"errors"
	"paymentgateway/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrEmailTaken = errors.New("email_taken")
var ErrNotFound = errors.New("merchant_not_found")

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db} }

func (r *Repository) Create(ctx context.Context, m *domain.Merchant) error {
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

func (r *Repository) FindByID(ctx context.Context, id string) (*domain.Merchant, error) {
	var m domain.Merchant
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, webhook_url, created_at FROM merchants WHERE id=$1`, id).
		Scan(&m.ID, &m.Name, &m.Email, &m.WebhookURL, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &m, err
}

func (r *Repository) List(ctx context.Context, limit, offset int) ([]domain.Merchant, int, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, email, webhook_url, created_at
		 FROM merchants
		 ORDER BY created_at DESC, id ASC
		 LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	merchants := []domain.Merchant{}
	for rows.Next() {
		var m domain.Merchant
		if err := rows.Scan(&m.ID, &m.Name, &m.Email, &m.WebhookURL, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		merchants = append(merchants, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM merchants`).Scan(&total); err != nil {
		return nil, 0, err
	}
	return merchants, total, nil
}

func (r *Repository) Update(ctx context.Context, id string, name, email, webhook *string) (*domain.Merchant, error) {
	query := `UPDATE merchants
	          SET name = COALESCE($2, name),
	              email = COALESCE($3, email)`
	args := []any{id, name, email}

	if webhook != nil {
		query += `, webhook_url = $4`
		var value any
		if *webhook != "" {
			value = *webhook
		}
		args = append(args, value)
	}
	query += ` WHERE id=$1
	           RETURNING id, name, email, webhook_url, created_at`

	var m domain.Merchant
	err := r.db.QueryRowContext(ctx, query, args...).
		Scan(&m.ID, &m.Name, &m.Email, &m.WebhookURL, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return &m, nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM merchants WHERE id=$1`, id)
	return err
}
