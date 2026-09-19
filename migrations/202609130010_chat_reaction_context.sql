-- +goose Up
CREATE TABLE chat_reaction_context (
  id BIGSERIAL PRIMARY KEY,
  telegram_chat_id BIGINT NOT NULL,
  author TEXT NOT NULL DEFAULT '',
  text TEXT NOT NULL CHECK (char_length(text) BETWEEN 1 AND 500),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX chat_reaction_context_recent ON chat_reaction_context(telegram_chat_id, id DESC);

-- Keep only a small rolling context per chat; this is not a chat archive.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION trim_chat_reaction_context() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  DELETE FROM chat_reaction_context
  WHERE telegram_chat_id = NEW.telegram_chat_id
    AND id <= COALESCE((SELECT id FROM chat_reaction_context WHERE telegram_chat_id=NEW.telegram_chat_id ORDER BY id DESC OFFSET 30 LIMIT 1), 0);
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER chat_reaction_context_trim AFTER INSERT ON chat_reaction_context
FOR EACH ROW EXECUTE FUNCTION trim_chat_reaction_context();

-- +goose Down
DROP TABLE IF EXISTS chat_reaction_context;
DROP FUNCTION IF EXISTS trim_chat_reaction_context();
