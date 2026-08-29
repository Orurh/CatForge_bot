-- +goose Up
CREATE TABLE IF NOT EXISTS yard_event_results (
  event_id        BIGINT PRIMARY KEY REFERENCES yard_events(id) ON DELETE CASCADE,
  success         BOOLEAN NOT NULL,
  team_score      INTEGER NOT NULL CHECK (team_score >= 0),
  target_score    INTEGER NOT NULL CHECK (target_score > 0),
  fish_total      INTEGER NOT NULL CHECK (fish_total >= 0),
  secret_found    BOOLEAN NOT NULL,
  strategy_bonus  INTEGER NOT NULL CHECK (strategy_bonus >= 0),
  rules_version   INTEGER NOT NULL,
  content_version INTEGER NOT NULL,
  resolved_at     TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS yard_event_participant_results (
  event_id      BIGINT NOT NULL REFERENCES yard_event_results(event_id) ON DELETE CASCADE,
  cat_id        BIGINT NOT NULL REFERENCES cats(id) ON DELETE CASCADE,
  choice_id     TEXT NOT NULL CHECK (choice_id IN ('steal', 'distract', 'scout')),
  contribution INTEGER NOT NULL CHECK (contribution >= 0),
  fish_reward   INTEGER NOT NULL CHECK (fish_reward >= 0),
  mvp           BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (event_id, cat_id)
);

-- +goose Down
DROP TABLE IF EXISTS yard_event_participant_results;
DROP TABLE IF EXISTS yard_event_results;
