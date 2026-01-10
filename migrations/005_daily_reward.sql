-- +goose Up

ALTER TABLE users
	ADD COLUMN IF NOT EXISTS daily_claim_date date;

ALTER TABLE users
	ADD COLUMN IF NOT EXISTS daily_streak int NOT NULL DEFAULT 0;

-- +goose Down

ALTER TABLE users
	DROP COLUMN IF EXISTS daily_claim_date;

ALTER TABLE users
	DROP COLUMN IF EXISTS daily_streak;