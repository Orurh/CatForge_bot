-- +goose Up
ALTER TABLE cats
  ADD COLUMN IF NOT EXISTS last_train_at TIMESTAMPTZ NOT NULL DEFAULT 'epoch',
  ADD COLUMN IF NOT EXISTS energy_updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE cats
SET energy_updated_at = COALESCE(energy_updated_at, updated_at, created_at, now())
WHERE energy_updated_at IS NULL;

-- +goose Down
ALTER TABLE cats
  DROP COLUMN IF EXISTS last_train_at,
  DROP COLUMN IF EXISTS energy_updated_at;