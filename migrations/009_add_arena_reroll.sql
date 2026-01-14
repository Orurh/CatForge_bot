-- +goose Up
ALTER TABLE arena_state
  ADD COLUMN IF NOT EXISTS reroll_day date,
  ADD COLUMN IF NOT EXISTS rerolls_today int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS reroll_ready_at timestamptz;

-- +goose Down
ALTER TABLE arena_state
  DROP COLUMN IF EXISTS reroll_ready_at,
  DROP COLUMN IF EXISTS rerolls_today,
  DROP COLUMN IF EXISTS reroll_day;