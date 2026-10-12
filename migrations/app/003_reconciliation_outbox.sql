CREATE TABLE IF NOT EXISTS reconciliation_outbox (
  payment_id UUID PRIMARY KEY REFERENCES payments(id),
  idempotency_key TEXT NOT NULL,
  step TEXT NOT NULL,
  error TEXT NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
