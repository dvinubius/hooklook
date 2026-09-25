#!/usr/bin/env bash

set -euo pipefail

project_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$project_dir"

mode=${1:-full}
timeout_seconds=${SMOKE_TEST_TIMEOUT_SECONDS:-45}
if [[ $mode != full && $mode != dashboard ]]; then
  printf '%s\n' 'Usage: telemetry-smoke-test.sh [full|dashboard]' >&2
  exit 1
fi
if ! [[ $timeout_seconds =~ ^[1-9][0-9]*$ ]]; then
  printf '%s\n' 'SMOKE_TEST_TIMEOUT_SECONDS must be a positive integer.' >&2
  exit 1
fi
if [[ ! -f .env.observability ]]; then
  printf '%s\n' '.env.observability is required.' >&2
  exit 1
fi

grafana_user=$(sed -n 's/^GRAFANA_ADMIN_USER=//p' .env.observability)
grafana_password=$(sed -n 's/^GRAFANA_ADMIN_PASSWORD=//p' .env.observability)
if [[ ! $grafana_user =~ ^[A-Za-z0-9_-]+$ || ! $grafana_password =~ ^[A-Za-z0-9_-]{24,}$ ]]; then
  printf '%s\n' 'Valid Hooklook Grafana credentials are missing from .env.observability.' >&2
  exit 1
fi

compose=(scripts/compose.sh)
grafana_id=$("${compose[@]}" ps --status running -q grafana)
if [[ -z $grafana_id ]]; then
  printf '%s\n' 'grafana is not running.' >&2
  exit 1
fi
# Compose v2.6 can report :0 for `compose port` even when Docker has published
# the configured address. Inspect the running container's actual port binding.
grafana_binding=$(docker inspect --format '{{range (index .NetworkSettings.Ports "3000/tcp")}}{{.HostIp}}:{{.HostPort}}{{end}}' "$grafana_id")
if [[ $grafana_binding != 127.0.0.1:3001 ]]; then
  printf 'Grafana must bind only to 127.0.0.1:3001; got %s.\n' "$grafana_binding" >&2
  exit 1
fi
if [[ $mode == full ]]; then
  for service in hooklook prometheus alloy loki; do
    [[ -n $("${compose[@]}" ps --status running -q "$service") ]] || {
      printf '%s is not running.\n' "$service" >&2
      exit 1
    }
  done
fi

wait_for() {
  local description=$1
  shift
  local attempt=1
  while (( attempt <= timeout_seconds )); do
    if "$@"; then
      printf 'PASS: %s\n' "$description"
      return 0
    fi
    sleep 1
    ((attempt += 1))
  done
  printf 'FAIL: %s did not pass within %s seconds.\n' "$description" "$timeout_seconds" >&2
  return 1
}

grafana_get() {
  # Keep the credential out of curl's process arguments and diagnostic output.
  printf 'user = "%s:%s"\n' "$grafana_user" "$grafana_password" |
    curl --config - --fail --silent "$@"
}

grafana_healthy() {
  curl --fail --silent http://127.0.0.1:3001/api/health >/dev/null
}

dashboard_ready() {
  grafana_get http://127.0.0.1:3001/api/dashboards/uid/hooklook-operator |
    grep -q '"uid":"hooklook-operator"'
}

wait_for 'Grafana HTTP API is healthy' grafana_healthy
if [[ $mode == dashboard ]]; then
  # File provisioning polls every 10 seconds by default. Allow one poll before
  # checking the dashboard API so an old copy does not satisfy the check.
  sleep 12
  wait_for 'Hooklook dashboard is provisioned' dashboard_ready
  printf '%s\n' 'Dashboard smoke test passed.'
  exit 0
fi

prometheus_target_up() {
  grafana_get --get --data-urlencode 'query=up{job="hooklook"} == 1' \
    http://127.0.0.1:3001/api/datasources/proxy/uid/hooklook-prometheus/api/v1/query |
    grep -q '"result":\[{'
}

loki_has_hooklook_log() {
  grafana_get --get --data-urlencode 'query={service="hooklook"}' \
    --data-urlencode 'limit=1' \
    http://127.0.0.1:3001/api/datasources/proxy/uid/hooklook-loki/loki/api/v1/query_range |
    grep -q '"result":\[{'
}

datasource_healthy() {
  grafana_get "http://127.0.0.1:3001/api/datasources/uid/$1/health" |
    grep -q '"status":"OK"'
}

curl --fail --silent http://127.0.0.1:8081/ready >/dev/null
curl --fail --silent http://127.0.0.1:8081/health >/dev/null
printf '%s\n' 'PASS: Hooklook is live and SQLite is ready.'
wait_for 'Prometheus reports Hooklook as up' prometheus_target_up
wait_for 'Loki contains a Hooklook application log' loki_has_hooklook_log
wait_for 'Grafana Prometheus datasource is healthy' datasource_healthy hooklook-prometheus
wait_for 'Grafana Loki datasource is healthy' datasource_healthy hooklook-loki
wait_for 'Hooklook dashboard is provisioned' dashboard_ready
printf '%s\n' 'Telemetry smoke test passed.'
