-- +goose Up
ALTER TABLE yard_auto_message_usage
  ADD COLUMN IF NOT EXISTS single_used INTEGER NOT NULL DEFAULT 0 CHECK (single_used BETWEEN 0 AND 1),
  ADD COLUMN IF NOT EXISTS banter_used INTEGER NOT NULL DEFAULT 0 CHECK (banter_used BETWEEN 0 AND 1);

-- Usage before this migration came only from single-cat Yard Event reactions.
UPDATE yard_auto_message_usage
SET single_used = LEAST(used, 1)
WHERE used > 0 AND single_used = 0;

-- +goose Down
ALTER TABLE yard_auto_message_usage DROP COLUMN IF EXISTS banter_used;
ALTER TABLE yard_auto_message_usage DROP COLUMN IF EXISTS single_used;
