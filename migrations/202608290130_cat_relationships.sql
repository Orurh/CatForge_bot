-- +goose Up
CREATE TABLE IF NOT EXISTS cat_relationships (
  cat_a_id   BIGINT NOT NULL REFERENCES cats(id) ON DELETE CASCADE,
  cat_b_id   BIGINT NOT NULL REFERENCES cats(id) ON DELETE CASCADE,
  friendship INTEGER NOT NULL DEFAULT 0 CHECK (friendship >= 0),
  rivalry    INTEGER NOT NULL DEFAULT 0 CHECK (rivalry >= 0),
  respect    INTEGER NOT NULL DEFAULT 0 CHECK (respect >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (cat_a_id, cat_b_id),
  CHECK (cat_a_id < cat_b_id)
);

CREATE TABLE IF NOT EXISTS yard_event_relationship_effects (
  event_id         BIGINT NOT NULL REFERENCES yard_event_results(event_id) ON DELETE CASCADE,
  cat_a_id         BIGINT NOT NULL REFERENCES cats(id) ON DELETE CASCADE,
  cat_b_id         BIGINT NOT NULL REFERENCES cats(id) ON DELETE CASCADE,
  friendship_delta INTEGER NOT NULL DEFAULT 0,
  rivalry_delta    INTEGER NOT NULL DEFAULT 0,
  respect_delta    INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (event_id, cat_a_id, cat_b_id),
  CHECK (cat_a_id < cat_b_id)
);

-- +goose Down
DROP TABLE IF EXISTS yard_event_relationship_effects;
DROP TABLE IF EXISTS cat_relationships;
