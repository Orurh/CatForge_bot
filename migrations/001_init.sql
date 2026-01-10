-- +goose Up
CREATE TABLE IF NOT EXISTS users (
  id           BIGSERIAL PRIMARY KEY,
  telegram_id  BIGINT UNIQUE NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS cats (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  breed      TEXT NOT NULL,
  trait      TEXT NOT NULL,
  level      INT  NOT NULL DEFAULT 1,
  xp         BIGINT NOT NULL DEFAULT 0,
  energy     INT  NOT NULL DEFAULT 100,
  hp_base    INT  NOT NULL,
  atk_base   INT  NOT NULL,
  def_base   INT  NOT NULL,
  spd_base   INT  NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);

-- +goose Down
DROP TABLE IF EXISTS cats;
DROP TABLE IF EXISTS users;
