-- +goose Up
CREATE TABLE yard_bedtime_schedule (
    yard_id BIGINT PRIMARY KEY REFERENCES yards(id) ON DELETE CASCADE,
    sleep_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE yard_bedtime_schedule;
