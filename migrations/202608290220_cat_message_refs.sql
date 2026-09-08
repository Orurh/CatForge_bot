-- +goose Up
CREATE TABLE IF NOT EXISTS cat_message_refs (
    telegram_chat_id bigint NOT NULL,
    telegram_message_id bigint NOT NULL CHECK (telegram_message_id > 0),
    cat_id bigint NOT NULL REFERENCES cats(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    claimed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (telegram_chat_id, telegram_message_id)
);

CREATE INDEX IF NOT EXISTS cat_message_refs_expires_at_idx
    ON cat_message_refs (expires_at);

-- +goose Down
DROP TABLE IF EXISTS cat_message_refs;
