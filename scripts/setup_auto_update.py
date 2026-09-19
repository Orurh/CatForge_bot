#!/usr/bin/env python3
"""Install the current user's CatForge deployment timer on the server."""
import json
import os
from pathlib import Path
import subprocess


def quote(value):
    if '\n' in value or '\r' in value:
        raise ValueError('Newlines in paths are unsupported')
    return '"' + value.replace('\\', '\\\\').replace('"', '\\"').replace('%', '%%') + '"'


def main():
    root = Path(__file__).resolve().parent.parent
    config = json.loads((root / '.deploy/host.json').read_text())
    if not config.get('repository'):
        raise SystemExit('Set repository in .deploy/host.json first')
    units = Path(os.environ.get('XDG_CONFIG_HOME', str(Path.home() / '.config'))) / 'systemd/user'
    units.mkdir(parents=True, exist_ok=True)
    (units / 'catforge-deploy.service').write_text(f'''[Unit]
Description=Deploy CatForge main after successful GitHub CI

[Service]
Type=oneshot
WorkingDirectory={quote(str(root))}
Environment=GIT_TERMINAL_PROMPT=0
Environment="GIT_SSH_COMMAND=ssh -o BatchMode=yes"
EnvironmentFile=-%h/.config/catforge-deploy.env
ExecStart=/usr/bin/python3 {quote(str(root / 'scripts/auto_update.py'))}
UMask=0077
TimeoutStartSec=45min
''')
    (units / 'catforge-deploy.timer').write_text('''[Unit]
Description=Check CatForge main every five minutes

[Timer]
OnBootSec=2min
OnUnitInactiveSec=5min
RandomizedDelaySec=30s

[Install]
WantedBy=timers.target
''')
    subprocess.run(['systemd-analyze', '--user', 'verify', str(units / 'catforge-deploy.service'),
                    str(units / 'catforge-deploy.timer')], check=True)
    subprocess.run(['systemctl', '--user', 'daemon-reload'], check=True)
    subprocess.run(['loginctl', 'enable-linger', os.environ['USER']], check=True)
    subprocess.run(['systemctl', '--user', 'enable', '--now', 'catforge-deploy.timer'], check=True)


if __name__ == '__main__':
    main()
