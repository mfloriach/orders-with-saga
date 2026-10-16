CREATE TABLE IF NOT EXISTS compensation_outbox (
  payment_id UUID PRIMARY KEY REFERENCES payments(id),
  saga_id TEXT NOT NULL,
  saga_name TEXT NOT NULL,
  completed TEXT[] NOT NULL,
  failed_step TEXT NOT NULL,
  step_error TEXT NOT NULL,
  claimed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
