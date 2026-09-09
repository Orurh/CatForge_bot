-- +goose Up
CREATE OR REPLACE VIEW analytics_daily_game_events AS
SELECT
  occurred_at::date AS event_date,
  event_type,
  rules_version,
  content_version,
  notable,
  count(*) AS event_count,
  count(DISTINCT user_id) AS unique_users
FROM game_events
GROUP BY occurred_at::date, event_type, rules_version, content_version, notable;

CREATE OR REPLACE VIEW analytics_activation_funnel AS
SELECT
  count(DISTINCT user_id) FILTER (WHERE event_type = 'user_started') AS started_users,
  count(DISTINCT user_id) FILTER (WHERE event_type = 'cat_created') AS cat_created_users,
  count(DISTINCT user_id) FILTER (WHERE event_type IN ('cat_trained', 'hunt_completed')) AS trained_users,
  count(DISTINCT user_id) FILTER (
    WHERE event_type = 'cat_trained'
      AND COALESCE(NULLIF(properties->>'target_chat_id', '')::BIGINT, 0) <> 0
  ) AS first_social_action_users
FROM game_events;

-- +goose Down
DROP VIEW IF EXISTS analytics_activation_funnel;
DROP VIEW IF EXISTS analytics_daily_game_events;
