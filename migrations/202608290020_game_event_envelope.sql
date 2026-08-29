-- +goose Up
ALTER TABLE game_events
  ADD COLUMN IF NOT EXISTS event_key TEXT,
  ADD COLUMN IF NOT EXISTS cat_id BIGINT REFERENCES cats(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS yard_id BIGINT,
  ADD COLUMN IF NOT EXISTS rules_version INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS content_version INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS payload_version INTEGER NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS notable BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX IF NOT EXISTS idx_game_events_event_key
  ON game_events (event_key) WHERE event_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_game_events_cat_time
  ON game_events (cat_id, occurred_at DESC) WHERE cat_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_game_events_yard_time
  ON game_events (yard_id, occurred_at DESC) WHERE yard_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_game_events_yard_time;
DROP INDEX IF EXISTS idx_game_events_cat_time;
DROP INDEX IF EXISTS idx_game_events_event_key;

ALTER TABLE game_events
  DROP COLUMN IF EXISTS notable,
  DROP COLUMN IF EXISTS payload_version,
  DROP COLUMN IF EXISTS content_version,
  DROP COLUMN IF EXISTS rules_version,
  DROP COLUMN IF EXISTS yard_id,
  DROP COLUMN IF EXISTS cat_id,
  DROP COLUMN IF EXISTS event_key;
