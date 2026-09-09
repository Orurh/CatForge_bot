-- +goose Up
CREATE VIEW analytics_ai_life_daily AS
SELECT
  occurred_at::date AS event_date,
  yard_id,
  count(*) FILTER (
    WHERE event_type = 'cat_reply_generated'
      AND COALESCE((properties->>'requested')::boolean, true)
  ) AS requested_cat_replies,
  count(*) FILTER (WHERE event_type = 'autonomous_cat_message') AS autonomous_messages,
  count(*) FILTER (WHERE event_type = 'cat_banter') AS banter_conversations,
  count(*) FILTER (WHERE event_type = 'human_reply_to_cat') AS human_replies_to_cat,
  count(*) FILTER (WHERE event_type = 'reaction_to_cat') AS cat_followups,
  count(*) FILTER (
    WHERE event_type = 'cat_autospeak_changed'
      AND COALESCE((properties->>'enabled')::boolean, false) = false
  ) AS autospeak_disabled,
  count(*) FILTER (
    WHERE event_type = 'yard_settings_changed'
      AND COALESCE((properties->>'auto_messages_enabled')::boolean, false) = false
  ) AS yard_auto_disabled,
  count(*) FILTER (
    WHERE event_type = 'yard_settings_changed'
      AND COALESCE((properties->>'cat_to_cat_banter')::boolean, false) = false
  ) AS yard_banter_disabled,
  count(*) FILTER (
    WHERE event_type = 'yard_settings_changed'
      AND COALESCE((properties->>'fights_enabled')::boolean, false) = false
  ) AS yard_fights_disabled,
  count(*) FILTER (
    WHERE event_type = 'yard_settings_changed'
      AND NULLIF(properties->>'quiet_until', '')::timestamptz > occurred_at
  ) AS quiet_enabled,
  count(*) FILTER (
    WHERE event_type IN (
      'cat_reply_generated', 'autonomous_cat_message', 'cat_banter', 'reaction_to_cat'
    ) AND COALESCE((properties->>'fallback')::boolean, false)
  ) AS fallback_outputs,
  count(DISTINCT cat_id) FILTER (WHERE cat_id IS NOT NULL) AS speaking_cats
FROM game_events
WHERE event_type IN (
  'cat_reply_generated',
  'autonomous_cat_message',
  'cat_banter',
  'human_reply_to_cat',
  'reaction_to_cat',
  'cat_autospeak_changed',
  'yard_settings_changed'
)
GROUP BY occurred_at::date, yard_id;

-- +goose Down
DROP VIEW IF EXISTS analytics_ai_life_daily;
