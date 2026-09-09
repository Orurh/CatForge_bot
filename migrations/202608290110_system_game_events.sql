-- +goose Up
ALTER TABLE game_events ALTER COLUMN user_id DROP NOT NULL;

-- +goose Down
DELETE FROM game_events WHERE user_id IS NULL;
ALTER TABLE game_events ALTER COLUMN user_id SET NOT NULL;
