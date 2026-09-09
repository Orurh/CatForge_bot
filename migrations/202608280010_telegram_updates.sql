-- +goose Up
CREATE TABLE IF NOT EXISTS telegram_updates (
  update_id   BIGINT PRIMARY KEY,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS telegram_updates;
