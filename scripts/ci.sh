#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
export CI_IMAGE_TAG
CI_IMAGE_TAG="$(git rev-parse HEAD)"
compose=(docker compose --env-file .env.example -p "catforge-ci-$$" -f deploy/ci/compose.yml)
cleanup() {
  status=$?
  trap - EXIT
  if (( status != 0 )); then
    "${compose[@]}" logs --tail=80 || true
  fi
  # This project contains only disposable test data, never the live database.
  "${compose[@]}" down --volumes --remove-orphans || true
  exit "$status"
}
trap cleanup EXIT

labels=(--label "org.opencontainers.image.revision=$CI_IMAGE_TAG")
if [[ -n "${GITHUB_REPOSITORY:-}" ]]; then
  labels+=(--label "org.opencontainers.image.source=${GITHUB_SERVER_URL:-https://github.com}/$GITHUB_REPOSITORY")
fi
docker build -f engine_cpp/Dockerfile "${labels[@]}" \
  -t "catforge-ci-engine:$CI_IMAGE_TAG" .
docker build "${labels[@]}" \
  -t "catforge-ci-bot:$CI_IMAGE_TAG" .
"${compose[@]}" up -d --wait --wait-timeout 120
export CATFORGE_TEST_DATABASE_URL CATFORGE_TEST_ENGINE
database_address="$("${compose[@]}" port postgres 5432)"
CATFORGE_TEST_DATABASE_URL="postgres://catforge_ci:catforge_ci@$database_address/catforge_ci?sslmode=disable"
CATFORGE_TEST_ENGINE="$("${compose[@]}" port game-engine 50051)"
python3 - <<'PY'
import os, socket, time
host, port = os.environ['CATFORGE_TEST_ENGINE'].rsplit(':', 1)
for attempt in range(60):
    try:
        with socket.create_connection((host, int(port)), timeout=1):
            break
    except OSError:
        time.sleep(1)
else:
    raise SystemExit('Test engine did not start')
PY
# The environment above is mandatory: integration tests must not silently skip.
go test -race -count=1 ./...
go vet ./...
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts -p 'test_*.py'

for base in docker-compose.dev.yml docker-compose.yml; do
  docker compose --env-file .env.example -f "$base" config --quiet
  docker compose --env-file .env.example -f "$base" -f docker-compose.monitoring.yml config --quiet
done
docker run --rm --network none -v "$PWD/deploy/monitoring:/etc/prometheus:ro" \
  --entrypoint /bin/promtool prom/prometheus:v3.14.0 check config /etc/prometheus/prometheus.yml
docker run --rm --network none -v "$PWD/deploy/monitoring:/tests:ro" -w /tests \
  --entrypoint /bin/promtool prom/prometheus:v3.14.0 test rules alerts_test.yml
