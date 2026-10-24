-- +goose Up
CREATE TABLE IF NOT EXISTS outbox_payments (
  id UUID PRIMARY KEY,
  payment_id UUID NOT NULL UNIQUE REFERENCES payments(id),
  merchant_id UUID NOT NULL REFERENCES merchants(id),
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processed','failed')),
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS outbox_payments_status_pending_idx ON outbox_payments (status) WHERE status = 'pending';

-- +goose Down
DROP TABLE IF EXISTS outbox_payments;
