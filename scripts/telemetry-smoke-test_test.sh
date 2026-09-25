#!/usr/bin/env bash
set -euo pipefail

project_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
temporary_dir=$(mktemp -d)
trap 'rm -rf "$temporary_dir"' EXIT
mkdir -p "$temporary_dir/project/scripts" "$temporary_dir/bin"
cp "$project_dir/scripts/telemetry-smoke-test.sh" "$project_dir/scripts/compose.sh" "$temporary_dir/project/scripts/"
printf '%s\n' 'HOOKLOOK_IMAGE=ghcr.io/example/hooklook@sha256:0000' >"$temporary_dir/project/.env.image"
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
    case ${SMOKE_TEST_PROMETHEUS:-up} in
      up) printf '%s\n' '{"data":{"activeTargets":[{"scrapeUrl":"http://hooklook:9092/metrics","health":"up"}]}}' ;;
      # A changed target is down; the old series still answers `up == 1`.
      down) printf '%s\n' '{"data":{"activeTargets":[{"scrapeUrl":"http://hooklook:9999/metrics","health":"down"}],"result":[{}]}}' ;;
      mixed) printf '%s\n' '{"data":{"activeTargets":[{"health":"up"},{"health":"unknown"}]}}' ;;
      none) printf '%s\n' '{"data":{"activeTargets":[]}}' ;;
    esac ;;
  *'/api/datasources/proxy/uid/hooklook-loki/'*)
    [[ ${SMOKE_TEST_LOKI:-new} == new ]] && printf '%s\n' '{"data":{"result":[{}]}}' || printf '%s\n' '{"data":{"result":[]}}' ;;
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
grep -q 'hooklook-prometheus/api/v1/targets' "$temporary_dir/smoke.log"
grep -q 'scrapePool=hooklook' "$temporary_dir/smoke.log"
grep -q 'hooklook-loki/loki/api/v1/query_range' "$temporary_dir/smoke.log"
grep -qE ' start=[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z .*hooklook-loki/' "$temporary_dir/smoke.log"
! grep -q ' port grafana 3000' "$temporary_dir/smoke.log"
grep -q ' inspect --format ' "$temporary_dir/smoke.log"
grep -q 'compose --env-file .env.observability --env-file .env.image --profile observability ps' "$temporary_dir/smoke.log"

: >"$temporary_dir/smoke.log"
run_smoke dashboard >"$temporary_dir/output"
grep -q 'Dashboard smoke test passed.' "$temporary_dir/output"
! grep -q 'hooklook-prometheus/' "$temporary_dir/smoke.log"
! grep -q '/ready' "$temporary_dir/smoke.log"

for prometheus_state in down mixed none; do
  : >"$temporary_dir/smoke.log"
  if PATH="$temporary_dir/bin:$PATH" SMOKE_TEST_LOG="$temporary_dir/smoke.log" \
    SMOKE_TEST_PROMETHEUS=$prometheus_state SMOKE_TEST_TIMEOUT_SECONDS=1 \
    bash "$temporary_dir/project/scripts/telemetry-smoke-test.sh" full >"$temporary_dir/output" 2>&1; then
    printf 'Telemetry smoke test accepted Prometheus targets: %s.\n' "$prometheus_state" >&2
    exit 1
  fi
  grep -q 'Prometheus reports Hooklook as up did not pass' "$temporary_dir/output"
done

# No Hooklook line since the test started: the current pipeline shipped nothing.
: >"$temporary_dir/smoke.log"
if PATH="$temporary_dir/bin:$PATH" SMOKE_TEST_LOG="$temporary_dir/smoke.log" \
  SMOKE_TEST_LOKI=old SMOKE_TEST_TIMEOUT_SECONDS=1 \
  bash "$temporary_dir/project/scripts/telemetry-smoke-test.sh" full >"$temporary_dir/output" 2>&1; then
  printf '%s\n' 'Telemetry smoke test accepted Loki without a new Hooklook log.' >&2
  exit 1
fi
grep -q 'Loki received a Hooklook log from this test did not pass' "$temporary_dir/output"

if PATH="$temporary_dir/bin:$PATH" SMOKE_TEST_LOG="$temporary_dir/smoke.log" \
  SMOKE_TEST_BAD_BIND=1 \
  bash "$temporary_dir/project/scripts/telemetry-smoke-test.sh" dashboard >"$temporary_dir/output" 2>&1; then
  printf '%s\n' 'Telemetry smoke test accepted a public Grafana bind.' >&2
  exit 1
fi
grep -q 'Grafana must bind only to 127.0.0.1:3001' "$temporary_dir/output"
printf '%s\n' 'telemetry smoke script tests passed'
