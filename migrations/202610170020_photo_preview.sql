-- +goose Up
ALTER TABLE pending_inputs ADD COLUMN payload TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE pending_inputs DROP COLUMN payload;
