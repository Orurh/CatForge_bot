-- +goose Up
ALTER TABLE cats
  ADD COLUMN IF NOT EXISTS name text NOT NULL DEFAULT 'Мурчалкин000';

-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint c
    WHERE c.conname = 'cats_name_len'
      AND c.conrelid = 'cats'::regclass
  ) THEN
    ALTER TABLE cats
      ADD CONSTRAINT cats_name_len
      CHECK (char_length(name) BETWEEN 1 AND 24);
  END IF;
END
$$;
-- +goose StatementEnd

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS pending_action text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users
  DROP COLUMN IF EXISTS pending_action;

ALTER TABLE cats
  DROP CONSTRAINT IF EXISTS cats_name_len;

ALTER TABLE cats
  DROP COLUMN IF EXISTS name;