-- +goose Up
ALTER TABLE yards
  ADD COLUMN IF NOT EXISTS fights_enabled BOOLEAN NOT NULL DEFAULT TRUE;

DROP VIEW IF EXISTS analytics_yard_auto_usage;
CREATE VIEW analytics_yard_auto_usage AS
SELECT
  y.id AS yard_id,
  y.auto_messages_enabled,
  y.max_auto_messages_day,
  y.cat_to_cat_banter,
  y.quiet_until,
  u.usage_day,
  COALESCE(u.used, 0) AS messages_claimed,
  COALESCE(u.single_used, 0) AS single_messages_claimed,
  COALESCE(u.banter_used, 0) AS banter_messages_claimed,
  y.fights_enabled
FROM yards y
LEFT JOIN yard_auto_message_usage u ON u.yard_id = y.id;

CREATE VIEW analytics_cat_banter_daily AS
SELECT
  occurred_at::date AS event_date,
  yard_id,
  properties->>'trigger' AS trigger,
  count(*) AS conversations_sent,
  count(*) FILTER (WHERE COALESCE((properties->>'fallback')::boolean, false)) AS fallback_conversations
FROM game_events
WHERE event_type = 'cat_banter'
GROUP BY occurred_at::date, yard_id, properties->>'trigger';

-- +goose Down
DROP VIEW IF EXISTS analytics_cat_banter_daily;
DROP VIEW IF EXISTS analytics_yard_auto_usage;
CREATE VIEW analytics_yard_auto_usage AS
SELECT
  y.id AS yard_id,
  y.auto_messages_enabled,
  y.max_auto_messages_day,
  y.cat_to_cat_banter,
  y.quiet_until,
  u.usage_day,
  COALESCE(u.used, 0) AS messages_claimed
FROM yards y
LEFT JOIN yard_auto_message_usage u ON u.yard_id = y.id;

ALTER TABLE yards DROP COLUMN IF EXISTS fights_enabled;
