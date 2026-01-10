-- +goose Up
ALTER TABLE arena_state
  ADD COLUMN IF NOT EXISTS rage int NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE arena_state
  DROP COLUMN IF EXISTS rage;
