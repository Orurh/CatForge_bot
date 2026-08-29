-- +goose Up
CREATE TABLE IF NOT EXISTS ai_rate_limits (
  scope_type  TEXT NOT NULL CHECK (scope_type IN ('user', 'chat')),
  scope_id    BIGINT NOT NULL,
  bucket_start TIMESTAMPTZ NOT NULL,
  used        INTEGER NOT NULL CHECK (used > 0),
  PRIMARY KEY (scope_type, scope_id, bucket_start)
);

CREATE INDEX IF NOT EXISTS idx_ai_rate_limits_bucket
  ON ai_rate_limits (bucket_start);

-- +goose Down
DROP TABLE IF EXISTS ai_rate_limits;
