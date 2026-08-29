-- +goose Up
CREATE TABLE IF NOT EXISTS pending_inputs (
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    telegram_chat_id bigint NOT NULL,
    kind text NOT NULL,
    prompt_message_id bigint NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, telegram_chat_id),
    CHECK (kind <> ''),
    CHECK (prompt_message_id > 0)
);

CREATE INDEX IF NOT EXISTS pending_inputs_expires_at_idx
    ON pending_inputs (expires_at);

-- +goose Down
DROP TABLE IF EXISTS pending_inputs;
