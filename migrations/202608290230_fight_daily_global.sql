-- +goose Up
CREATE INDEX IF NOT EXISTS yard_fights_cat_a_daily_idx
    ON yard_fights (cat_a_id, created_at DESC);

CREATE INDEX IF NOT EXISTS yard_fights_cat_b_daily_idx
    ON yard_fights (cat_b_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS yard_fights_cat_b_daily_idx;
DROP INDEX IF EXISTS yard_fights_cat_a_daily_idx;
