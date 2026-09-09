-- +goose Up
-- Human activity only. Durable fight/choice tables avoid winner-only counting
-- and repeated choice updates counting as a second event.
CREATE VIEW analytics_beta_actions AS
SELECT e.user_id, c.id AS cat_id, e.yard_id, e.occurred_at,
       CASE e.event_type WHEN 'cat_trained' THEN 'training' ELSE 'conversation' END AS action,
       'game_event:' || e.id AS action_key
FROM game_events e
LEFT JOIN cats c ON c.user_id = e.user_id
WHERE e.user_id IS NOT NULL AND e.occurred_at <= now()
  AND (e.event_type IN ('cat_trained', 'human_reply_to_cat')
       OR (e.event_type = 'cat_reply_generated' AND COALESCE(e.properties->>'requested', 'true') = 'true'))
UNION ALL
SELECT c.user_id, c.id, f.yard_id, f.created_at, 'fight', 'fight:' || f.id
FROM yard_fights f
CROSS JOIN LATERAL (VALUES (f.cat_a_id), (f.cat_b_id)) participant(cat_id)
JOIN cats c ON c.id = participant.cat_id
WHERE f.created_at <= now()
UNION ALL
SELECT c.user_id, c.id, e.yard_id, choice.submitted_at, 'event', 'event:' || e.id
FROM yard_event_choices choice
JOIN yard_events e ON e.id = choice.event_id
JOIN cats c ON c.id = choice.cat_id
WHERE e.state <> 'cancelled' AND choice.submitted_at <= now();

CREATE VIEW analytics_beta_user_cohorts AS
WITH first_cat AS (
  SELECT user_id, (min(occurred_at) AT TIME ZONE 'Europe/Moscow')::date AS cohort_date
  FROM game_events WHERE event_type = 'cat_created' AND user_id IS NOT NULL AND occurred_at <= now()
  GROUP BY user_id
), activity AS (
  SELECT DISTINCT user_id, (occurred_at AT TIME ZONE 'Europe/Moscow')::date AS active_date
  FROM analytics_beta_actions
), members AS (
  SELECT f.*, d.day_number,
    f.cohort_date + d.day_number < (now() AT TIME ZONE 'Europe/Moscow')::date AS eligible,
    a.user_id IS NOT NULL AS returned
  FROM first_cat f CROSS JOIN (VALUES (1), (3), (7), (14)) d(day_number)
  LEFT JOIN activity a ON a.user_id = f.user_id AND a.active_date = f.cohort_date + d.day_number
)
SELECT cohort_date, day_number, count(*) AS cohort_users,
       count(*) FILTER (WHERE eligible) AS eligible_users,
       count(*) FILTER (WHERE eligible AND returned) AS returned_users,
       round(100.0 * count(*) FILTER (WHERE eligible AND returned)
             / nullif(count(*) FILTER (WHERE eligible), 0), 1) AS retention_pct
FROM members GROUP BY cohort_date, day_number;

CREATE VIEW analytics_beta_repeat_actions AS
WITH ranked AS (
  SELECT user_id, action, occurred_at,
    row_number() OVER (PARTITION BY user_id, action ORDER BY occurred_at, action_key) AS n
  FROM analytics_beta_actions WHERE action IN ('training', 'fight', 'event')
)
SELECT user_id, action, min(occurred_at) AS first_at,
       min(occurred_at) FILTER (WHERE n = 2) AS second_at, count(*) AS action_count
FROM ranked GROUP BY user_id, action;

CREATE VIEW analytics_beta_yard_daily AS
SELECT yard_id, (occurred_at AT TIME ZONE 'Europe/Moscow')::date AS activity_date,
       count(DISTINCT cat_id) AS active_cats, count(DISTINCT user_id) AS active_users,
       count(*) FILTER (WHERE action = 'training') AS trainings,
       count(DISTINCT action_key) FILTER (WHERE action = 'fight') AS fights,
       count(DISTINCT action_key) FILTER (WHERE action = 'event') AS events
FROM analytics_beta_actions WHERE yard_id IS NOT NULL
GROUP BY yard_id, (occurred_at AT TIME ZONE 'Europe/Moscow')::date;

CREATE VIEW analytics_beta_yard_activation AS
WITH activated AS (
  SELECT yard_id, min(activity_date) AS activated_date
  FROM analytics_beta_yard_daily WHERE active_cats >= 3 GROUP BY yard_id
)
SELECT y.id AS yard_id, (y.created_at AT TIME ZONE 'Europe/Moscow')::date AS created_date,
       a.activated_date,
       COALESCE(a.activated_date + 7 < (now() AT TIME ZONE 'Europe/Moscow')::date, false) AS d7_eligible,
       CASE WHEN a.activated_date + 7 < (now() AT TIME ZONE 'Europe/Moscow')::date
            THEN COALESCE(d.active_cats, 0) >= 3 END AS three_active_cats_d7
FROM yards y LEFT JOIN activated a ON a.yard_id = y.id
LEFT JOIN analytics_beta_yard_daily d ON d.yard_id = y.id AND d.activity_date = a.activated_date + 7;

-- +goose Down
DROP VIEW IF EXISTS analytics_beta_yard_activation;
DROP VIEW IF EXISTS analytics_beta_yard_daily;
DROP VIEW IF EXISTS analytics_beta_repeat_actions;
DROP VIEW IF EXISTS analytics_beta_user_cohorts;
DROP VIEW IF EXISTS analytics_beta_actions;
