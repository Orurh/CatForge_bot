-- +goose Up
DROP VIEW IF EXISTS analytics_yard_social_summary;
DROP VIEW IF EXISTS analytics_yard_event_metrics;

ALTER TABLE yard_event_results
  RENAME COLUMN fish_total TO yard_score;

ALTER TABLE yard_event_results
  ADD COLUMN outcome_tier TEXT,
  ADD COLUMN participant_xp BIGINT NOT NULL DEFAULT 0 CHECK (participant_xp >= 0);

UPDATE yard_event_results
SET outcome_tier = CASE WHEN success THEN 'success' ELSE 'failure' END;

ALTER TABLE yard_event_results
  ALTER COLUMN outcome_tier SET NOT NULL,
  ADD CONSTRAINT yard_event_results_outcome_tier_check
    CHECK (outcome_tier IN ('failure', 'partial', 'success', 'exceptional'));

ALTER TABLE yard_event_participant_results
  DROP COLUMN fish_reward;

ALTER TABLE yard_fights
  ADD COLUMN cat_a_xp_gain BIGINT NOT NULL DEFAULT 0 CHECK (cat_a_xp_gain >= 0),
  ADD COLUMN cat_b_xp_gain BIGINT NOT NULL DEFAULT 0 CHECK (cat_b_xp_gain >= 0);

CREATE TABLE command_daily_usage (
  user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  telegram_chat_id BIGINT NOT NULL,
  command_name     TEXT NOT NULL,
  usage_day        DATE NOT NULL,
  used_at          TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (user_id, telegram_chat_id, command_name, usage_day)
);

CREATE INDEX command_daily_usage_day_idx
  ON command_daily_usage (usage_day);

CREATE VIEW analytics_yard_event_metrics AS
WITH choices AS (
  SELECT event_id, count(*) AS participants,
    count(*) FILTER (WHERE choice_id = 'steal') AS chose_steal,
    count(*) FILTER (WHERE choice_id = 'distract') AS chose_distract,
    count(*) FILTER (WHERE choice_id = 'scout') AS chose_scout
  FROM yard_event_choices
  GROUP BY event_id
), relationship_changes AS (
  SELECT event_id, sum(friendship_delta) AS friendship_gained,
    sum(rivalry_delta) AS rivalry_gained, sum(respect_delta) AS respect_gained
  FROM yard_event_relationship_effects
  GROUP BY event_id
)
SELECT e.id AS event_id, e.yard_id, e.event_type, e.state, e.starts_at,
  e.resolves_at, r.rules_version, e.content_version,
  COALESCE(c.participants, 0) AS participants,
  COALESCE(c.chose_steal, 0) AS chose_steal,
  COALESCE(c.chose_distract, 0) AS chose_distract,
  COALESCE(c.chose_scout, 0) AS chose_scout,
  r.outcome_tier, r.team_score, r.target_score, r.yard_score,
  r.participant_xp, r.secret_found,
  COALESCE(rc.friendship_gained, 0) AS friendship_gained,
  COALESCE(rc.rivalry_gained, 0) AS rivalry_gained,
  COALESCE(rc.respect_gained, 0) AS respect_gained
FROM yard_events e
LEFT JOIN choices c ON c.event_id = e.id
LEFT JOIN yard_event_results r ON r.event_id = e.id
LEFT JOIN relationship_changes rc ON rc.event_id = e.id;

CREATE VIEW analytics_yard_social_summary AS
SELECT yard_id, count(*) AS events_started,
  count(*) FILTER (WHERE state = 'resolved') AS events_resolved,
  count(*) FILTER (WHERE participants > 0) AS events_with_participation,
  count(*) FILTER (WHERE outcome_tier IN ('success', 'exceptional')) AS successful_events,
  sum(participants) AS total_choices,
  count(DISTINCT event_id) FILTER (WHERE participants >= 2) AS social_events,
  sum(friendship_gained) AS friendship_gained,
  sum(rivalry_gained) AS rivalry_gained,
  sum(respect_gained) AS respect_gained
FROM analytics_yard_event_metrics
GROUP BY yard_id;

-- +goose Down
DROP VIEW IF EXISTS analytics_yard_social_summary;
DROP VIEW IF EXISTS analytics_yard_event_metrics;

DROP TABLE IF EXISTS command_daily_usage;

ALTER TABLE yard_fights
  DROP COLUMN IF EXISTS cat_b_xp_gain,
  DROP COLUMN IF EXISTS cat_a_xp_gain;

ALTER TABLE yard_event_participant_results
  ADD COLUMN fish_reward INTEGER NOT NULL DEFAULT 0 CHECK (fish_reward >= 0);

ALTER TABLE yard_event_results
  DROP CONSTRAINT IF EXISTS yard_event_results_outcome_tier_check,
  DROP COLUMN IF EXISTS participant_xp,
  DROP COLUMN IF EXISTS outcome_tier;

ALTER TABLE yard_event_results
  RENAME COLUMN yard_score TO fish_total;

CREATE VIEW analytics_yard_event_metrics AS
WITH choices AS (
  SELECT event_id, count(*) AS participants,
    count(*) FILTER (WHERE choice_id = 'steal') AS chose_steal,
    count(*) FILTER (WHERE choice_id = 'distract') AS chose_distract,
    count(*) FILTER (WHERE choice_id = 'scout') AS chose_scout
  FROM yard_event_choices
  GROUP BY event_id
), relationship_changes AS (
  SELECT event_id, sum(friendship_delta) AS friendship_gained,
    sum(rivalry_delta) AS rivalry_gained, sum(respect_delta) AS respect_gained
  FROM yard_event_relationship_effects
  GROUP BY event_id
)
SELECT e.id AS event_id, e.yard_id, e.event_type, e.state, e.starts_at,
  e.resolves_at, r.rules_version, e.content_version,
  COALESCE(c.participants, 0) AS participants,
  COALESCE(c.chose_steal, 0) AS chose_steal,
  COALESCE(c.chose_distract, 0) AS chose_distract,
  COALESCE(c.chose_scout, 0) AS chose_scout,
  r.success, r.team_score, r.target_score, r.fish_total, r.secret_found,
  COALESCE(rc.friendship_gained, 0) AS friendship_gained,
  COALESCE(rc.rivalry_gained, 0) AS rivalry_gained,
  COALESCE(rc.respect_gained, 0) AS respect_gained
FROM yard_events e
LEFT JOIN choices c ON c.event_id = e.id
LEFT JOIN yard_event_results r ON r.event_id = e.id
LEFT JOIN relationship_changes rc ON rc.event_id = e.id;

CREATE VIEW analytics_yard_social_summary AS
SELECT yard_id, count(*) AS events_started,
  count(*) FILTER (WHERE state = 'resolved') AS events_resolved,
  count(*) FILTER (WHERE participants > 0) AS events_with_participation,
  count(*) FILTER (WHERE success) AS successful_events,
  sum(participants) AS total_choices,
  count(DISTINCT event_id) FILTER (WHERE participants >= 2) AS social_events,
  sum(friendship_gained) AS friendship_gained,
  sum(rivalry_gained) AS rivalry_gained,
  sum(respect_gained) AS respect_gained
FROM analytics_yard_event_metrics
GROUP BY yard_id;
