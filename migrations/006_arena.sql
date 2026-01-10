-- +goose Up
CREATE TABLE IF NOT EXISTS arena_state (
  user_id            BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  tickets            INT    NOT NULL,
  tickets_updated_at TIMESTAMPTZ NOT NULL,
  rating             INT    NOT NULL DEFAULT 1000,
  season_points      INT    NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS arena_matches (
  id             BIGSERIAL PRIMARY KEY,
  attacker_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  defender_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  seed           TEXT NOT NULL,
  attacker_power INT  NOT NULL,
  defender_power INT  NOT NULL,
  attacker_won   BOOLEAN NOT NULL,
  rating_delta   INT NOT NULL
);

CREATE INDEX IF NOT EXISTS arena_matches_attacker_created_idx ON arena_matches(attacker_id, created_at DESC);
CREATE INDEX IF NOT EXISTS arena_matches_defender_created_idx ON arena_matches(defender_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS arena_matches;
DROP TABLE IF EXISTS arena_state;
