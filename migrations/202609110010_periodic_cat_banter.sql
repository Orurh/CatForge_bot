-- +goose Up
CREATE TABLE yard_banter_schedule (
  yard_id BIGINT PRIMARY KEY REFERENCES yards(id) ON DELETE CASCADE,
  last_attempt TIMESTAMPTZ NOT NULL
);
ALTER TABLE yards ALTER COLUMN auto_messages_enabled SET DEFAULT TRUE;
ALTER TABLE yards ALTER COLUMN cat_to_cat_banter SET DEFAULT TRUE;
-- Enable untouched defaults; preserve yards whose settings were edited.
UPDATE yards SET auto_messages_enabled = TRUE, cat_to_cat_banter = TRUE
WHERE updated_at = created_at;

ALTER TABLE cat_personality ALTER COLUMN auto_speak_enabled SET DEFAULT TRUE;
UPDATE cat_personality SET auto_speak_enabled = TRUE WHERE updated_at = created_at;

-- +goose Down
ALTER TABLE cat_personality ALTER COLUMN auto_speak_enabled SET DEFAULT FALSE;
ALTER TABLE yards ALTER COLUMN auto_messages_enabled SET DEFAULT FALSE;
ALTER TABLE yards ALTER COLUMN cat_to_cat_banter SET DEFAULT FALSE;
DROP TABLE yard_banter_schedule;
