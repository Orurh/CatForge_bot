-- +goose Up
ALTER TABLE yards
  ADD COLUMN IF NOT EXISTS quiet_until TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS yard_auto_message_usage (
  yard_id    BIGINT NOT NULL REFERENCES yards(id) ON DELETE CASCADE,
  usage_day  DATE NOT NULL,
  used       INTEGER NOT NULL DEFAULT 0 CHECK (used BETWEEN 0 AND 2),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (yard_id, usage_day)
);

-- +goose Down
DROP TABLE IF EXISTS yard_auto_message_usage;
ALTER TABLE yards DROP COLUMN IF EXISTS quiet_until;
