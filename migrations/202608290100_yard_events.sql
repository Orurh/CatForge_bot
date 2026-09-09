-- +goose Up
CREATE TABLE IF NOT EXISTS yard_events (
  id              BIGSERIAL PRIMARY KEY,
  yard_id         BIGINT NOT NULL REFERENCES yards(id) ON DELETE CASCADE,
  event_type      TEXT NOT NULL,
  state           TEXT NOT NULL CHECK (state IN ('active', 'resolved', 'cancelled')),
  seed            BIGINT NOT NULL,
  starts_at       TIMESTAMPTZ NOT NULL,
  resolves_at     TIMESTAMPTZ NOT NULL,
  content_version INTEGER NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (resolves_at > starts_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_yard_events_one_active
  ON yard_events (yard_id) WHERE state = 'active';

CREATE INDEX IF NOT EXISTS idx_yard_events_resolve
  ON yard_events (state, resolves_at) WHERE state = 'active';

CREATE TABLE IF NOT EXISTS yard_event_choices (
  event_id     BIGINT NOT NULL REFERENCES yard_events(id) ON DELETE CASCADE,
  cat_id       BIGINT NOT NULL REFERENCES cats(id) ON DELETE CASCADE,
  choice_id    TEXT NOT NULL CHECK (choice_id IN ('steal', 'distract', 'scout')),
  submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (event_id, cat_id)
);

CREATE INDEX IF NOT EXISTS idx_yard_event_choices_choice
  ON yard_event_choices (event_id, choice_id);

-- +goose Down
DROP TABLE IF EXISTS yard_event_choices;
DROP TABLE IF EXISTS yard_events;
