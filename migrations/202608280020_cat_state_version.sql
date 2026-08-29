-- +goose Up
ALTER TABLE cats
  ADD COLUMN IF NOT EXISTS state_version BIGINT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE cats
  DROP COLUMN IF EXISTS state_version;
