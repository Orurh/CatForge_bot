#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${1:-docker-compose.dev.yml}"
cd "$project_root"
docker compose -f "$compose_file" config --quiet
# Units belong to the invoking user; no root service or broad system changes.
python3 - "$project_root" "$compose_file" <<'PY'
import os
from pathlib import Path
import sys

root, compose = sys.argv[1:]
def quote(value):
    if '\n' in value or '\r' in value:
        raise SystemExit('Newlines in paths are not supported')
    return '"' + value.replace('\\', '\\\\').replace('"', '\\"').replace('%', '%%') + '"'

units = Path(os.environ.get('XDG_CONFIG_HOME', str(Path.home() / '.config'))) / 'systemd/user'
units.mkdir(parents=True, exist_ok=True)
(units / 'catforge-backup.service').write_text(f'''[Unit]
Description=CatForge daily PostgreSQL backup
StartLimitIntervalSec=0

[Service]
Type=oneshot
Environment={quote('COMPOSE_FILE=' + compose)}
EnvironmentFile=-%h/.config/catforge-backup.env
ExecStart=/usr/bin/bash {quote(root + '/scripts/backup_daily.sh')}
UMask=0077
TimeoutStartSec=15min
Restart=on-failure
RestartSec=15min
''')
(units / 'catforge-backup.timer').write_text('''[Unit]
Description=Daily CatForge backup with catch-up after downtime

[Timer]
OnCalendar=*-*-* 04:00:00 Europe/Moscow
Persistent=true
RandomizedDelaySec=5min

[Install]
WantedBy=timers.target
''')
print('CatForge backup units installed in', units)
PY
systemd-analyze --user verify "${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/catforge-backup.service" "${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/catforge-backup.timer"
systemctl --user daemon-reload
loginctl enable-linger "$(id -un)"
systemctl --user enable --now catforge-backup.timer
systemctl --user start catforge-backup.service
systemctl --user list-timers catforge-backup.timer --no-pager
