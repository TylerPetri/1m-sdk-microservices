#!/usr/bin/env bash
set -euo pipefail
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root_dir"
work_dir=$(mktemp -d)
project="sdkms-smoke-$(date +%s)-$$"
export AUTH_JWT_SECRET
AUTH_JWT_SECRET=$(python3 -c 'import secrets; print(secrets.token_hex(32))')
export COMPOSE_PROJECT_NAME="$project"
log_dir="${SDK_SMOKE_LOG_DIR:-$work_dir/logs}"
mkdir -p "$log_dir"
python3 - "$work_dir/override.yml" <<'PY'
import json,os,sys
ports={'postgres':[], 'authd':[8081], 'hellod':[8081], 'gatewayd':[8080,8081],
       'prometheus':[9090], 'grafana':[3000], 'loki':[3100], 'otel-collector':[4318], 'promtail':[]}
s='services:\n'
for service,values in ports.items():
 s+='  '+service+':\n    ports: !override '+json.dumps(['127.0.0.1::'+str(p) for p in values])+'\n'
 s+='    environment:\n      NO_PROXY: "localhost,127.0.0.1,postgres,authd,hellod,gatewayd,prometheus,grafana,loki,otel-collector"\n      no_proxy: "localhost,127.0.0.1,postgres,authd,hellod,gatewayd,prometheus,grafana,loki,otel-collector"\n'
ca=os.environ.get('CODEX_PROXY_CERT')
if ca:
 for service in ['authd','hellod','gatewayd']:
  # Repeated keys would be invalid YAML: append build settings to their existing block.
  marker='  '+service+':\n'
  s=s.replace(marker,marker+'    build:\n      secrets: [proxy_ca]\n',1)
 s+='secrets:\n  proxy_ca:\n    file: '+json.dumps(ca)+'\n'
open(sys.argv[1],'w').write(s)
PY
compose=(docker compose -p "$project" -f "$root_dir/deployments/docker-compose.yml" -f "$work_dir/override.yml" --profile prod --profile otel --profile logs)
cleanup() {
  result=$?
  trap - EXIT
  "${compose[@]}" logs --no-color > "$log_dir/compose.log" 2>&1 || true
  "${compose[@]}" ps -a > "$log_dir/containers.txt" 2>&1 || true
  "${compose[@]}" down -v --remove-orphans > "$log_dir/cleanup.log" 2>&1 || true
  rm -f "$work_dir/override.yml"
  echo "Compose smoke logs: $log_dir"
  exit "$result"
}
trap cleanup EXIT
"${compose[@]}" config --quiet
"${compose[@]}" build authd hellod gatewayd
"${compose[@]}" up -d --wait --wait-timeout 180
endpoint() { printf 'http://%s' "$("${compose[@]}" port "$1" "$2")"; }
export GATEWAY_URL GATEWAY_ADMIN_URL AUTH_ADMIN_URL HELLO_ADMIN_URL PROMETHEUS_URL GRAFANA_URL LOKI_URL OTEL_URL
GATEWAY_URL=$(endpoint gatewayd 8080)
GATEWAY_ADMIN_URL=$(endpoint gatewayd 8081)
AUTH_ADMIN_URL=$(endpoint authd 8081)
HELLO_ADMIN_URL=$(endpoint hellod 8081)
PROMETHEUS_URL=$(endpoint prometheus 9090)
GRAFANA_URL=$(endpoint grafana 3000)
LOKI_URL=$(endpoint loki 3100)
OTEL_URL=$(endpoint otel-collector 4318)
python3 scripts/ci/compose-smoke.py
