-- +goose Up
-- Older cat_trained events stored the source group only in the JSON payload.
-- Backfill the relational yard_id so weekly standings remain scoped to the
-- Yard where the training was shown. Private training has no target_chat_id
-- and intentionally stays unscoped.
UPDATE game_events AS ge
SET yard_id = y.id
FROM yards AS y
WHERE ge.event_type = 'cat_trained'
  AND ge.yard_id IS NULL
  AND ge.properties->>'target_chat_id' ~ '^-?[0-9]+$'
  AND y.telegram_chat_id = (ge.properties->>'target_chat_id')::bigint;

-- +goose Down
-- This is a data backfill. Keeping the resolved yard_id on rollback is safe
-- and avoids erasing the scope written by newer application versions.
SELECT 1;
