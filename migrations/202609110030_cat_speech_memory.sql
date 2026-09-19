-- +goose Up
CREATE TABLE cat_speech_memory (
  id BIGSERIAL PRIMARY KEY,
  scope_key TEXT NOT NULL,
  cat_id BIGINT NOT NULL REFERENCES cats(id) ON DELETE CASCADE,
  other_cat_id BIGINT REFERENCES cats(id) ON DELETE CASCADE,
  cat_name TEXT NOT NULL,
  other_cat_name TEXT NOT NULL DEFAULT '',
  speech TEXT NOT NULL CHECK (char_length(speech) BETWEEN 1 AND 500),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX cat_speech_memory_scope ON cat_speech_memory(scope_key,id DESC);

-- +goose Down
DROP TABLE cat_speech_memory;
