#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
docker compose -f "${COMPOSE_FILE:-docker-compose.dev.yml}" exec -T postgres \
  sh -c 'exec psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
BEGIN READ ONLY;
SET LOCAL statement_timeout = '30s';
SELECT * FROM analytics_beta_user_cohorts ORDER BY cohort_date, day_number;
SELECT action, count(*) AS first_action_users,
       count(second_at) AS second_action_users
FROM analytics_beta_repeat_actions GROUP BY action ORDER BY action;
SELECT count(*) AS total_yards, count(activated_date) AS activated_yards,
       count(*) FILTER (WHERE d7_eligible) AS d7_eligible_yards,
       count(*) FILTER (WHERE three_active_cats_d7) AS yard_3_active_cats_d7
FROM analytics_beta_yard_activation;
SELECT activity_date, count(*) AS active_yards,
       count(*) FILTER (WHERE active_cats >= 3) AS yards_with_3_active_cats
FROM analytics_beta_yard_daily GROUP BY activity_date ORDER BY activity_date;
COMMIT;
SQL
