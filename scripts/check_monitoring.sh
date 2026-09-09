#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
compose=(docker compose -f "${COMPOSE_FILE:-docker-compose.dev.yml}" -f docker-compose.monitoring.yml)
"${compose[@]}" config --quiet
"${compose[@]}" run --rm --no-deps --entrypoint /bin/promtool prometheus check config /etc/prometheus/prometheus.yml
"${compose[@]}" run --rm --no-deps --entrypoint /bin/promtool -v "$PWD/deploy/monitoring:/tests:ro" -w /tests prometheus test rules alerts_test.yml
"${compose[@]}" run --rm --no-deps --entrypoint /bin/amtool alertmanager check-config /etc/alertmanager/alertmanager.yml
"${compose[@]}" run --rm --no-deps alloy validate /etc/alloy/config.alloy
"${compose[@]}" run --rm --no-deps loki -config.file=/etc/loki/config.yml -verify-config=true
