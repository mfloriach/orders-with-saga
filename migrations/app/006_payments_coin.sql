-- +goose Up
ALTER TABLE payments ADD COLUMN coin TEXT NOT NULL DEFAULT 'USD';

-- +goose Down
ALTER TABLE payments DROP COLUMN IF EXISTS coin;
