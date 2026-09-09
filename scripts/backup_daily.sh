#!/usr/bin/env bash
set -euo pipefail
umask 077
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_root"
export COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.dev.yml}"
mkdir -p backups
# A manual run and the timer must not create competing archives.
exec 9>backups/.daily.lock
flock -n 9 || exit 0
output="backups/daily-$(date -u +%Y%m%dT%H%M%SZ).dump"
bash scripts/backup_db.sh "$output"

# Optional mounted off-host storage. Never silently replace a missing mount
# with a local directory; the local archive is retained on copy failure.
if [[ -n "${BACKUP_OFFSITE_DIR:-}" ]]; then
  if [[ ! -d "$BACKUP_OFFSITE_DIR" ]] || ! mountpoint -q "$BACKUP_OFFSITE_DIR"; then
    echo 'BACKUP_OFFSITE_DIR must be an existing mount point; local backup retained.' >&2
    exit 1
  fi
  destination="$BACKUP_OFFSITE_DIR/$(basename "$output")"
  temporary="$(mktemp "$BACKUP_OFFSITE_DIR/.catforge-backup.XXXXXX")"
  trap 'rm -f "$temporary"' EXIT
  cat "$output" > "$temporary"
  chmod 600 "$temporary"
  cmp -- "$output" "$temporary"
  ln "$temporary" "$destination"
  echo 'Off-host copy verified.'
fi
