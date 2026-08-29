-- +goose Up
CREATE INDEX IF NOT EXISTS idx_yard_events_yard_history
  ON yard_events (yard_id, id);

CREATE INDEX IF NOT EXISTS idx_yard_event_results_resolved_at
  ON yard_event_results (resolved_at, event_id);

-- +goose Down
DROP INDEX IF EXISTS idx_yard_event_results_resolved_at;
DROP INDEX IF EXISTS idx_yard_events_yard_history;
