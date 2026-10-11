CREATE TABLE IF NOT EXISTS ledger_entries (
  id UUID PRIMARY KEY,
  payment_id UUID NOT NULL UNIQUE,
  amount BIGINT NOT NULL CHECK (amount > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
