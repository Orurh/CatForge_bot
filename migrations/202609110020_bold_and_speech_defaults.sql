-- +goose Up
ALTER TABLE yards ALTER COLUMN humor_mode SET DEFAULT 'bold';
ALTER TABLE yards ALTER COLUMN auto_messages_enabled SET DEFAULT TRUE;
ALTER TABLE yards ALTER COLUMN cat_to_cat_banter SET DEFAULT TRUE;
ALTER TABLE yards ALTER COLUMN max_auto_messages_day SET DEFAULT 2;
ALTER TABLE cat_personality ALTER COLUMN humor_mode SET DEFAULT 'bold';
ALTER TABLE cat_personality ALTER COLUMN auto_speak_enabled SET DEFAULT TRUE;

-- Explicit rollout to all existing yards and cats, including edited settings.
UPDATE yards
SET humor_mode = 'bold', auto_messages_enabled = TRUE,
    cat_to_cat_banter = TRUE, max_auto_messages_day = 2,
    quiet_until = NULL, updated_at = now();
UPDATE cat_personality
SET humor_mode = 'bold', auto_speak_enabled = TRUE, updated_at = now();

-- +goose Down
-- Earlier individual settings cannot be reconstructed; retain the current values.
ALTER TABLE yards ALTER COLUMN humor_mode SET DEFAULT 'normal';
ALTER TABLE cat_personality ALTER COLUMN humor_mode SET DEFAULT 'normal';
