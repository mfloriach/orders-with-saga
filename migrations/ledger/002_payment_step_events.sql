CREATE TABLE IF NOT EXISTS payment_step_events (
  id UUID PRIMARY KEY,
  payment_id UUID NOT NULL,
  saga_id TEXT NOT NULL,
  step TEXT NOT NULL,
  outcome TEXT NOT NULL CHECK (outcome IN ('success','failure')),
  reason TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS payment_step_events_payment_id_idx ON payment_step_events (payment_id);
