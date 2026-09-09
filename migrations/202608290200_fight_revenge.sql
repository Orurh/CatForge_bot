-- +goose Up
ALTER TABLE yard_fights
    ADD COLUMN IF NOT EXISTS fight_kind text NOT NULL DEFAULT 'regular',
    ADD COLUMN IF NOT EXISTS parent_fight_id bigint REFERENCES yard_fights(id) ON DELETE SET NULL;

ALTER TABLE yard_fights
    ADD CONSTRAINT yard_fights_kind_check CHECK (fight_kind IN ('regular', 'revenge'));

CREATE UNIQUE INDEX IF NOT EXISTS yard_fights_parent_unique
    ON yard_fights (parent_fight_id)
    WHERE parent_fight_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS yard_fights_cat_a_created_idx
    ON yard_fights (yard_id, cat_a_id, created_at DESC);

CREATE INDEX IF NOT EXISTS yard_fights_cat_b_created_idx
    ON yard_fights (yard_id, cat_b_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS yard_fights_cat_b_created_idx;
DROP INDEX IF EXISTS yard_fights_cat_a_created_idx;
DROP INDEX IF EXISTS yard_fights_parent_unique;
ALTER TABLE yard_fights DROP CONSTRAINT IF EXISTS yard_fights_kind_check;
ALTER TABLE yard_fights DROP COLUMN IF EXISTS parent_fight_id;
ALTER TABLE yard_fights DROP COLUMN IF EXISTS fight_kind;
