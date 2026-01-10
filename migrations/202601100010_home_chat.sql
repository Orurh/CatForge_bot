-- +goose Up
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS home_chat_id BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS home_chat_type TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS home_chat_bound_at TIMESTAMPTZ NULL,
  ADD COLUMN IF NOT EXISTS home_chat_last_log_at TIMESTAMPTZ NULL;

-- +goose Down
ALTER TABLE users
  DROP COLUMN IF EXISTS home_chat_last_log_at,
  DROP COLUMN IF EXISTS home_chat_bound_at,
  DROP COLUMN IF EXISTS home_chat_type,
  DROP COLUMN IF EXISTS home_chat_id;
