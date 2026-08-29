-- +goose Up
ALTER TABLE cat_personality
  ADD COLUMN IF NOT EXISTS humor_mode TEXT NOT NULL DEFAULT 'normal';

-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'cat_personality_humor_mode'
      AND conrelid = 'cat_personality'::regclass
  ) THEN
    ALTER TABLE cat_personality
      ADD CONSTRAINT cat_personality_humor_mode
      CHECK (humor_mode IN ('normal', 'bold'));
  END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE cat_personality
  DROP CONSTRAINT IF EXISTS cat_personality_humor_mode,
  DROP COLUMN IF EXISTS humor_mode;
