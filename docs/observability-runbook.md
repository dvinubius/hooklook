# Hooklook observability

This is a private, Hooklook-owned stack. The [observability overview](observability.md)
documents its topology, ports, signals, and dashboard. This runbook covers
validation and operation. Nothing here changes Caddy, zibs, or host
observability. Prepare locally; deploying is a separate operator action.

## Signals

`/health` is static liveness. `/ready` checks SQLite within 500 ms. Capacity
is separate: `hooklook_storage_bytes{kind="allocated"}` measures the SQLite
main file against `kind="budget"` (`MAX_STORE`, rounded to whole pages), and
`kind="reusable"` counts pages that SQLite can reuse. Deleting rows can raise
reusable bytes without shrinking allocated bytes. `main_file`, `wal`, and
`filesystem_available` show other pressure. `hooklook_storage_collection_available`
and `hooklook_active_bins_collection_available` are zero on collection failure;
the last gauges must not be interpreted as fresh then.

Capture results are `accepted`, `missing_bin`, `bin_full`, `store_full`, and
`internal_error`. A `507` is expected capacity pressure and is separate from
true server failures. `up` only means Prometheus can scrape the process; use
`/ready` for SQLite readiness. Caddy-generated `413`, `429`, and `431` are not
visible to Go; inspect Caddy access logs separately when investigating these.
No host metrics or tracing are part of this stack.

Prometheus generates `up{job="hooklook"}` for the `job_name: hooklook` scrape
in `observability/prometheus.yml`, which targets `hooklook:9092` every 15
seconds. It sets `up` to 1 when a scrape succeeds and 0 when it fails. Hooklook
does not emit this metric itself, and `up=1` does not prove SQLite readiness.

Successful create, capture, list, detail, owner delete, clear, and admin-list
operations have counters. The list and detail counters reflect successful API
reads; page views and SSE reconnects do not count. SSE duration is excluded
from ordinary HTTP latency. Logs are JSON with fixed route class, bounded
method/status, duration, and safe operational events. Captured payloads,
headers, cookies, invitation IDs, raw URLs, and token values are not logged.

## Local validation

Run `go test ./...` and `go vet ./...`. Do not run frontend browser tests for
this milestone. Validate Compose with placeholders, without starting services:

```sh
PUBLIC_BASE_URL=https://example.invalid ADMIN_TOKEN=placeholder \
  GRAFANA_ADMIN_USER=operator GRAFANA_ADMIN_PASSWORD=placeholder \
  docker compose --profile observability config --quiet
```

Check configurations with the pinned image versions:

```sh
docker run --rm --entrypoint /bin/promtool -v "$PWD/observability/prometheus.yml:/etc/prometheus/prometheus.yml:ro" \
  prom/prometheus:v3.5.0 check config /etc/prometheus/prometheus.yml
docker run --rm -v "$PWD/observability/alloy.alloy:/etc/alloy/config.alloy:ro" \
  grafana/alloy:v1.10.2 validate /etc/alloy/config.alloy
docker run --rm -v "$PWD/observability/loki.yml:/etc/loki/config.yml:ro" \
  grafana/loki:3.5.3 -config.file=/etc/loki/config.yml -verify-config=true
python3 -m json.tool observability/grafana/dashboards/hooklook.json >/dev/null
```

Pinned versions were checked against release dates before adding them:
Prometheus v3.5.0 (2025-07-14), Alloy v1.10.2 (2025-08-19), Loki 3.5.3
(2025-07-23), Grafana 12.1.1 (2025-08-13), and Go Prometheus client
v1.23.2 (2025-09-05). All predate the repository's age threshold.

## Deployment preparation and private access

Set unique `GRAFANA_ADMIN_USER` and `GRAFANA_ADMIN_PASSWORD` in the local
untracked deployment environment. Use at least 24 URL-safe characters for the
password. `DEPLOY_PROFILE=full ./scripts/deploy.sh` is the explicit full-stack
mode. It runs local Go tests first, updates Hooklook only, writes credentials
to a mode-600 `/opt/hooklook/.env.observability`, and starts the four telemetry
services and runs `telemetry-smoke-test.sh` after startup. The default
`core` mode updates only the app. Once the stack exists,
`DEPLOY_PROFILE=observability` syncs telemetry configuration and Compose,
updates Prometheus, Alloy, Loki, and Grafana, then runs the same smoke
checks. It also checks the app's Compose network attachment without rebuilding
the image; the first run after a network rename recreates the app container
using the existing image and data volume. `DEPLOY_PROFILE=dashboard` validates
and syncs only the dashboard JSON, touches it for Grafana polling, then verifies
the dashboard API. These two
update modes require only `DEPLOY_HOST` and use the existing remote
`.env.observability`; dashboard mode does not restart Hooklook. The existing
`hooklook-data` volume remains unchanged. No command in this runbook deploys
anything automatically.

Only Grafana publishes a port: `127.0.0.1:3001:3000`. Use an SSH tunnel from
the operator workstation, then open `http://127.0.0.1:3001`:

```sh
ssh -L 3001:127.0.0.1:3001 "$DEPLOY_USER@$DEPLOY_HOST"
```

Grafana joins both the internal observability network and a Grafana-only
non-internal access bridge. On the production Docker engine, a container
attached only to an internal bridge kept the requested port in
`HostConfig.PortBindings` but had `NetworkSettings.Ports` set to null; host port
3001 was closed. The access bridge allows Docker to publish the loopback port.
The smoke test checks the live binding and Grafana's HTTP health endpoint.

Prometheus keeps 14 days with a 2 GB cap; Loki keeps seven days. The app Docker
stdout log is rotated at 10 MB × 3 files. Prometheus, Alloy, Loki, and Grafana
have Hooklook-only named volumes and networks. Alloy has access to the Docker
socket for discovery; filtering by both Compose project and service limits
forwarded logs, but the socket itself remains privileged host access.
The Compose-owned networks are `hooklook_metrics`, `hooklook_observability`,
and `hooklook_grafana-access`; their short Compose keys avoid a duplicate
project prefix. Renaming creates new Docker networks, so deployment reconnects
the app to the new metrics bridge. The former networks can be removed after
all containers have moved off them. Zibs uses its own Compose project and
network names, so this migration does not change zibs connectivity.

## After deployment

The full and observability deployment modes run the Hooklook-only smoke test
automatically. To rerun it on the host, call
`./scripts/telemetry-smoke-test.sh` from `/opt/hooklook`. It checks SQLite
readiness, the Prometheus scrape, a fresh safe log in Loki, both Grafana
datasources, and the dashboard. Dashboard-only deployment runs the focused
`dashboard` smoke mode after allowing one provisioning poll. Then exercise a home visit, capture, missing-bin capture, list/detail API read,
SSE open/close, and a deliberate test-bin capacity rejection. Check their
bounded counters and private logs in the dashboard. Verify `507` appears in
the capacity panel, separately from the application 5xx panel. Confirm the
Grafana bind with the running container's Docker port bindings (the smoke
script checks `docker inspect`; some Compose versions report `:0` from
`docker compose port`) and ensure there is no Caddy route for `/metrics` or
Grafana. Recheck public Hooklook and zibs
behavior. Configure two alerts only when a notification destination exists:
SQLite/volume capacity approaching the limit and any `store_full` rejection.
The dashboard also shows scrape loss, repeated cleanup failures, and 5xx.

For rollback, stop only Hooklook telemetry containers with
`docker compose --profile observability stop prometheus alloy loki grafana`.
Keep their named volumes for investigation. If the app instrumentation itself
needs rollback, restore the previous Hooklook image using the deployment
runbook. Do not modify Caddy, zibs, or host monitoring.
