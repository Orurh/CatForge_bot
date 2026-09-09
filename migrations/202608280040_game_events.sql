-- +goose Up
CREATE TABLE IF NOT EXISTS game_events (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  event_type  TEXT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL,
  properties  JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_game_events_user_time
  ON game_events (user_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_game_events_type_time
  ON game_events (event_type, occurred_at DESC);

-- +goose Down
DROP TABLE IF EXISTS game_events;
