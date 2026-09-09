#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
python3 - <<'PY'
import base64,json,urllib.request,urllib.parse
from pathlib import Path

def get(url,auth=False):
 headers={}
 if auth:
  password=Path('.monitoring-secrets/grafana_password').read_text().strip()
  headers['Authorization']='Basic '+base64.b64encode(('admin:'+password).encode()).decode()
 with urllib.request.urlopen(urllib.request.Request(url,headers=headers),timeout=15) as r:
  return json.load(r)

def query(expr):
 data=get('http://127.0.0.1:9090/api/v1/query?'+urllib.parse.urlencode({'query':expr}))
 assert data['status']=='success', 'Prometheus query failed'
 return data['data']['result']

expected={'catforge','prometheus','alertmanager','node','loki','alloy','grafana','blackbox','readiness'}
up=query('up')
actual={s['metric']['job'] for s in up if float(s['value'][1])==1}
assert expected<=actual, 'Targets unavailable: '+str(sorted(expected-actual))
print('Prometheus: all 9 scrape jobs UP')
for expr,label in [('probe_success{job="readiness"}','readiness'),('catforge_dependency_up','dependencies'),('catforge_backup_directory_readable','backup directory')]:
 rows=query(expr)
 assert rows and all(float(r['value'][1])==1 for r in rows), label+' failed'
 print(label+': healthy')
assert query('catforge_backup_last_success_timestamp_seconds > 0'), 'No backup archive found'
assert query('catforge_operation_duration_seconds_count{component="engine"}'), 'Engine RPC metrics absent'
assert get('http://127.0.0.1:3000/api/health')['database']=='ok'
for uid in ['prometheus','loki']:
 health=get('http://127.0.0.1:3000/api/datasources/uid/'+uid+'/health',True)
 assert health.get('status')=='OK', uid+' datasource unhealthy'
print('Grafana: both metric/log datasources healthy')
dashboard=get('http://127.0.0.1:3000/api/dashboards/uid/catforge-overview',True)
assert dashboard['dashboard']['panels'], 'Dashboard missing'
print('Grafana: dashboard provisioned')
params=urllib.parse.urlencode({'query':'{application="catforge",service="bot"}','limit':5})
logs=get('http://127.0.0.1:3000/api/datasources/proxy/uid/loki/loki/api/v1/query_range?'+params,True)
assert logs.get('status')=='success' and logs['data']['result'], 'Bot logs have not reached Loki'
print('Loki: real bot logs received (contents not printed)')
get('http://127.0.0.1:9093/api/v2/status')
print('Alertmanager: API reachable; external notification delivery is a separate check')
PY
