#!/usr/bin/env bash
set -euo pipefail

project_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
temporary_dir=$(mktemp -d)
trap 'rm -rf "$temporary_dir"' EXIT
mkdir -p "$temporary_dir/project/scripts" "$temporary_dir/bin"
cp "$project_dir/scripts/telemetry-smoke-test.sh" "$temporary_dir/project/scripts/"
printf 'GRAFANA_ADMIN_USER=operator\nGRAFANA_ADMIN_PASSWORD=abcdefghijklmnopqrstuvwxyz123456\n' >"$temporary_dir/project/.env.observability"

cat >"$temporary_dir/bin/docker" <<'MOCK'
#!/usr/bin/env bash
printf 'docker %s\n' "$*" >>"$SMOKE_TEST_LOG"
if [[ " $* " == *' inspect --format '* ]]; then
  if [[ ${SMOKE_TEST_BAD_BIND:-0} == 1 ]]; then
    printf '%s\n' '0.0.0.0:3001'
  else
    printf '%s\n' '127.0.0.1:3001'
  fi
fi
if [[ " $* " == *' port grafana 3000 '* ]]; then
  printf '%s\n' ':0'
fi
if [[ " $* " == *' ps --status running -q '* ]]; then
  printf '%s\n' 'container-id'
fi
MOCK
cat >"$temporary_dir/bin/curl" <<'MOCK'
#!/usr/bin/env bash
printf 'curl %s\n' "$*" >>"$SMOKE_TEST_LOG"
if [[ " $* " == *' --config - '* ]]; then
  read -r config_line
  [[ $config_line == 'user = "operator:abcdefghijklmnopqrstuvwxyz123456"' ]] || exit 1
fi
case "$*" in
  *'/api/dashboards/uid/hooklook-operator'*) printf '%s\n' '{"dashboard":{"uid":"hooklook-operator"}}' ;;
  *'/api/datasources/proxy/uid/hooklook-prometheus/'*)
    [[ ${SMOKE_TEST_FAIL_PROMETHEUS:-0} == 0 ]] && printf '%s\n' '{"data":{"result":[{}]}}' || printf '%s\n' '{"data":{"result":[]}}' ;;
  *'/api/datasources/proxy/uid/hooklook-loki/'*) printf '%s\n' '{"data":{"result":[{}]}}' ;;
  *'/api/datasources/uid/'*) printf '%s\n' '{"status":"OK"}' ;;
esac
MOCK
cat >"$temporary_dir/bin/sleep" <<'MOCK'
#!/usr/bin/env bash
printf 'sleep %s\n' "$*" >>"$SMOKE_TEST_LOG"
MOCK
chmod +x "$temporary_dir/bin/"*

run_smoke() {
  PATH="$temporary_dir/bin:$PATH" SMOKE_TEST_LOG="$temporary_dir/smoke.log" \
    bash "$temporary_dir/project/scripts/telemetry-smoke-test.sh" "$@"
}

run_smoke full >"$temporary_dir/output"
grep -q 'Telemetry smoke test passed.' "$temporary_dir/output"
grep -q 'hooklook-prometheus/api/v1/query' "$temporary_dir/smoke.log"
grep -q 'hooklook-loki/loki/api/v1/query_range' "$temporary_dir/smoke.log"
! grep -q ' port grafana 3000' "$temporary_dir/smoke.log"
grep -q ' inspect --format ' "$temporary_dir/smoke.log"

: >"$temporary_dir/smoke.log"
run_smoke dashboard >"$temporary_dir/output"
grep -q 'Dashboard smoke test passed.' "$temporary_dir/output"
! grep -q 'hooklook-prometheus/api/v1/query' "$temporary_dir/smoke.log"
! grep -q '/ready' "$temporary_dir/smoke.log"

: >"$temporary_dir/smoke.log"
if PATH="$temporary_dir/bin:$PATH" SMOKE_TEST_LOG="$temporary_dir/smoke.log" \
  SMOKE_TEST_FAIL_PROMETHEUS=1 SMOKE_TEST_TIMEOUT_SECONDS=1 \
  bash "$temporary_dir/project/scripts/telemetry-smoke-test.sh" full >"$temporary_dir/output" 2>&1; then
  printf '%s\n' 'Telemetry smoke test accepted a down Prometheus target.' >&2
  exit 1
fi
grep -q 'Prometheus reports Hooklook as up did not pass' "$temporary_dir/output"

if PATH="$temporary_dir/bin:$PATH" SMOKE_TEST_LOG="$temporary_dir/smoke.log" \
  SMOKE_TEST_BAD_BIND=1 \
  bash "$temporary_dir/project/scripts/telemetry-smoke-test.sh" dashboard >"$temporary_dir/output" 2>&1; then
  printf '%s\n' 'Telemetry smoke test accepted a public Grafana bind.' >&2
  exit 1
fi
grep -q 'Grafana must bind only to 127.0.0.1:3001' "$temporary_dir/output"
printf '%s\n' 'telemetry smoke script tests passed'
