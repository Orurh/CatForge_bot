-- +goose Up
CREATE TABLE IF NOT EXISTS yard_fight_queue (
    yard_id bigint NOT NULL REFERENCES yards(id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cat_id bigint NOT NULL REFERENCES cats(id) ON DELETE CASCADE,
    queued_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (yard_id, cat_id),
    UNIQUE (yard_id, user_id),
    CHECK (expires_at > queued_at)
);

CREATE INDEX IF NOT EXISTS yard_fight_queue_match_idx
    ON yard_fight_queue (yard_id, queued_at, cat_id);
CREATE INDEX IF NOT EXISTS yard_fight_queue_expires_idx
    ON yard_fight_queue (expires_at);

CREATE TABLE IF NOT EXISTS yard_fights (
    id bigserial PRIMARY KEY,
    yard_id bigint NOT NULL REFERENCES yards(id) ON DELETE CASCADE,
    cat_a_id bigint REFERENCES cats(id) ON DELETE SET NULL,
    cat_b_id bigint REFERENCES cats(id) ON DELETE SET NULL,
    cat_a_name text NOT NULL,
    cat_b_name text NOT NULL,
    winner_cat_id bigint REFERENCES cats(id) ON DELETE SET NULL,
    loser_cat_id bigint REFERENCES cats(id) ON DELETE SET NULL,
    winner_name text NOT NULL,
    loser_name text NOT NULL,
    seed bigint NOT NULL CHECK (seed >= 0),
    rounds integer NOT NULL CHECK (rounds > 0),
    final_hp_a integer NOT NULL CHECK (final_hp_a >= 0),
    final_hp_b integer NOT NULL CHECK (final_hp_b >= 0),
    turns jsonb NOT NULL DEFAULT '[]'::jsonb,
    rivalry_delta integer NOT NULL DEFAULT 1 CHECK (rivalry_delta > 0),
    rules_version integer NOT NULL,
    content_version integer NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS yard_fights_yard_created_idx
    ON yard_fights (yard_id, created_at DESC);
CREATE INDEX IF NOT EXISTS yard_fights_pair_idx
    ON yard_fights (yard_id, cat_a_id, cat_b_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS yard_fights;
DROP TABLE IF EXISTS yard_fight_queue;
