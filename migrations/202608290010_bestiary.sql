-- +goose Up
CREATE TABLE IF NOT EXISTS cat_bestiary (
  user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  enemy_kind   TEXT NOT NULL,
  encounters   INT NOT NULL DEFAULT 0 CHECK (encounters >= 0),
  victories    INT NOT NULL DEFAULT 0 CHECK (victories >= 0 AND victories <= encounters),
  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, enemy_kind)
);

-- +goose Down
DROP TABLE IF EXISTS cat_bestiary;
