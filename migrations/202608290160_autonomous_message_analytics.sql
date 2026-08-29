-- +goose Up
CREATE OR REPLACE VIEW analytics_autonomous_messages_daily AS
SELECT
  occurred_at::date AS event_date,
  yard_id,
  count(*) AS messages_sent,
  count(DISTINCT cat_id) AS speaking_cats,
  count(*) FILTER (WHERE COALESCE((properties->>'fallback')::boolean, false)) AS fallback_messages
FROM game_events
WHERE event_type = 'autonomous_cat_message'
GROUP BY occurred_at::date, yard_id;

CREATE OR REPLACE VIEW analytics_yard_auto_usage AS
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

-- +goose Down
DROP VIEW IF EXISTS analytics_yard_auto_usage;
DROP VIEW IF EXISTS analytics_autonomous_messages_daily;
