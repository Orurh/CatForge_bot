-- +goose Up
CREATE TABLE IF NOT EXISTS yards (
  id                    BIGSERIAL PRIMARY KEY,
  telegram_chat_id      BIGINT UNIQUE NOT NULL,
  name                  TEXT NOT NULL,
  humor_mode            TEXT NOT NULL DEFAULT 'normal' CHECK (humor_mode IN ('normal', 'bold')),
  auto_messages_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  max_auto_messages_day INTEGER NOT NULL DEFAULT 2 CHECK (max_auto_messages_day BETWEEN 0 AND 2),
  cat_to_cat_banter     BOOLEAN NOT NULL DEFAULT FALSE,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS yard_members (
  yard_id        BIGINT NOT NULL REFERENCES yards(id) ON DELETE CASCADE,
  user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  cat_id         BIGINT NOT NULL REFERENCES cats(id) ON DELETE CASCADE,
  joined_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_active_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (yard_id, user_id),
  UNIQUE (yard_id, cat_id)
);

CREATE INDEX IF NOT EXISTS idx_yard_members_cat
  ON yard_members (cat_id);

ALTER TABLE game_events
  ADD CONSTRAINT game_events_yard_id_fkey
  FOREIGN KEY (yard_id) REFERENCES yards(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE game_events DROP CONSTRAINT IF EXISTS game_events_yard_id_fkey;
DROP TABLE IF EXISTS yard_members;
DROP TABLE IF EXISTS yards;
