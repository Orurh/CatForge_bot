#!/usr/bin/env bash
set -euo pipefail
umask 077
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"
compose_file="${COMPOSE_FILE:-docker-compose.yml}"
output="${1:?Usage: COMPOSE_FILE=docker-compose.dev.yml scripts/backup_db.sh /path/backup.dump}"
if [[ -e "$output" ]]; then
  echo 'Output already exists; choose a new backup path.' >&2
  exit 1
fi
temporary="$(mktemp "${output}.XXXXXX")"
trap 'rm -f "$temporary"' EXIT
docker compose -f "$compose_file" exec -T postgres sh -c 'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$temporary"
# Validate the archive before publishing it.
docker compose -f "$compose_file" exec -T postgres pg_restore --list < "$temporary" > /dev/null
ln "$temporary" "$output"
echo "Backup saved: $output"
