-- +goose Up
CREATE TABLE group_action_cooldowns (
 chat_id BIGINT NOT NULL,
 actor_id BIGINT NOT NULL,
 action TEXT NOT NULL,
 next_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(chat_id, actor_id, action)
);
-- One row per group/actor/action, not one row per attempted command.
-- +goose Down
DROP TABLE IF EXISTS group_action_cooldowns;
