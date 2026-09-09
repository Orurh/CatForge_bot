-- +goose Up
DROP VIEW IF EXISTS analytics_activation_funnel;
CREATE VIEW analytics_activation_funnel AS
SELECT
  count(DISTINCT user_id) FILTER (WHERE event_type = 'user_started') AS started_users,
  count(DISTINCT user_id) FILTER (WHERE event_type = 'cat_created') AS cat_created_users,
  count(DISTINCT user_id) FILTER (WHERE event_type = 'first_personality_line') AS first_personality_line_users,
  count(DISTINCT user_id) FILTER (WHERE event_type IN ('cat_trained', 'hunt_completed')) AS trained_users,
  count(DISTINCT user_id) FILTER (
    WHERE event_type = 'cat_trained'
      AND COALESCE(NULLIF(properties->>'target_chat_id', '')::BIGINT, 0) <> 0
  ) AS first_social_action_users,
  count(DISTINCT user_id) FILTER (WHERE event_type = 'yard_member_joined') AS joined_yard_users
FROM game_events;

-- +goose Down
DROP VIEW IF EXISTS analytics_activation_funnel;
CREATE VIEW analytics_activation_funnel AS
SELECT
  count(DISTINCT user_id) FILTER (WHERE event_type = 'user_started') AS started_users,
  count(DISTINCT user_id) FILTER (WHERE event_type = 'cat_created') AS cat_created_users,
  count(DISTINCT user_id) FILTER (WHERE event_type = 'first_personality_line') AS first_personality_line_users,
  count(DISTINCT user_id) FILTER (WHERE event_type IN ('cat_trained', 'hunt_completed')) AS trained_users,
  count(DISTINCT user_id) FILTER (
    WHERE event_type = 'cat_trained'
      AND COALESCE(NULLIF(properties->>'target_chat_id', '')::BIGINT, 0) <> 0
  ) AS first_social_action_users
FROM game_events;
