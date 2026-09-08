#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
python3 - <<'PY'
import os, secrets
from pathlib import Path
p=Path('.monitoring-secrets')
p.mkdir(mode=0o700,exist_ok=True)
p.chmod(0o700)
try:
 fd=os.open(p/'grafana_password',os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o444)
except FileExistsError:
 pass
else:
 with os.fdopen(fd,'w') as f: f.write(secrets.token_urlsafe(32)+'\n')
b=Path('backups'); b.mkdir(mode=0o750,exist_ok=True); b.chmod(0o750)
# Container may list/stat backups through the group, but archives stay mode 600.
print('Grafana login: admin; password file: .monitoring-secrets/grafana_password')
print('Run compose with MONITORING_BACKUP_GID='+str(b.stat().st_gid))
PY
