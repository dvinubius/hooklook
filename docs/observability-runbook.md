# Hooklook observability

This is a private, Hooklook-owned stack. The [observability overview](observability.md)
documents its topology, ports, signals, and dashboard. This runbook covers
validation and operation. Nothing here changes Caddy, zibs, or host
observability, except the public-dashboard route in
[Share only the public dashboard](#share-only-the-public-dashboard). Prepare
locally; deploying is a separate operator action.

## Signals

What each metric, `up`, `/health`, and `/ready` mean, and what never reaches
metrics or logs, is documented in the [observability overview](observability.md#goals-and-instrumentation).
The [dashboard guide](observability-guide.md) explains how to read them.

## Local validation

Run `go test ./...` and `go vet ./...`. Do not run frontend browser tests for
this milestone. Validate Compose with placeholders, without starting services:

```sh
PUBLIC_BASE_URL=https://example.invalid ADMIN_TOKEN=placeholder \
  GRAFANA_ADMIN_USER=operator GRAFANA_ADMIN_PASSWORD=placeholder \
  HOOKLOOK_IMAGE=ghcr.io/example/hooklook@sha256:$(printf '0%.0s' {1..64}) \
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
./scripts/ci-deploy.sh validate
```

Pinned versions were checked against release dates before adding them:
Prometheus v3.5.0 (2025-07-14), Alloy v1.10.2 (2025-08-19), Loki 3.5.3
(2025-07-23), Grafana 12.1.1 (2025-08-13), and Go Prometheus client
v1.23.2 (2025-09-05). All predate the repository's age threshold.

## Deployment preparation and private access

Grafana credentials live only in the VPS-owned, mode-0600
`/opt/hooklook/.env.observability` (`GRAFANA_ADMIN_USER`, and a
`GRAFANA_ADMIN_PASSWORD` of at least 24 URL-safe characters). Its presence is
what makes deployment manage the telemetry stack.

Pushes to `main` deploy through GitHub Actions; see the
[deployment runbook](deployment-runbook.md). A full deployment recreates the
app from its GHCR image, recreates the four telemetry services, and runs
`telemetry-smoke-test.sh`. A push that changes only `observability/` files
other than the dashboard installs that directory, recreates Prometheus, Alloy,
Loki, and Grafana, and runs the same smoke checks without pulling or
recreating the app. A push that changes only the dashboard JSON files
(`hooklook.json`, `traffic-synthetic.json`, `traffic-other.json`,
`public-metrics.json` in `observability/grafana/dashboards/`) installs those
files and checks all four dashboard UIDs after Grafana's polling interval,
with no Compose `up`.
Changing both the dashboard and other observability files is a full
deployment. The `hooklook-data` volume remains unchanged.

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
bounded counters and private logs in the dashboard; the
[dashboard guide](observability-guide.md#1-what-the-dashboard-can-see) shows
where each one appears. Verify `507` appears in
the capacity panel, separately from the application 5xx panel. Confirm the
Grafana bind with the running container's Docker port bindings (the smoke
script checks `docker inspect`; some Compose versions report `:0` from
`docker compose port`) and ensure Caddy has no route for `/metrics` and
forwards only the public-dashboard allowlist to Grafana. Recheck public Hooklook and zibs
behavior. Configure two alerts only when a notification destination exists:
SQLite/volume capacity approaching the limit and any `store_full` rejection.
The dashboard also shows scrape loss, repeated cleanup failures, and 5xx.

A failed telemetry deployment restores its snapshot automatically. To reverse a
successful one, use the snapshot rollback in the deployment runbook. To stop
only the Hooklook telemetry containers, run
`./scripts/compose.sh stop prometheus alloy loki grafana` from
`/opt/hooklook`, and keep their named volumes for investigation. Do not modify Caddy, zibs, or host monitoring.

## Share only the public dashboard

**Hooklook public metrics** is the only dashboard that may be shared. Never
share the operator or traffic-class dashboards: they contain logs and private
capacity detail.

The route has two halves, and both must be deployed before a share works:

1. This repository's `compose.yaml` attaches Grafana to `hooklook-edge` under
   the alias `hooklook-grafana`.
2. [hetzner-one](https://github.com/dvinubius/hetzner-one)'s `Caddyfile` sends
   the public-dashboard allowlist on `hooklook.app` to `hooklook-grafana:3000`.
   Its zibs route dials `zibs-grafana-1`, never the bare `grafana` name, which
   both Grafana containers carry on their edge networks. Keep it that way: a
   bare `grafana` upstream could send zibs's public-dashboard requests to
   Hooklook's Grafana.

Then, in Grafana (through the SSH tunnel):

1. Open **Hooklook → Hooklook public metrics** and review every panel's saved
   query against the [allowed content](observability.md#public-dashboard).
2. Use **Share → Share externally**, choose **Anyone with the link**, and
   leave time-range selection and annotations disabled.
3. Copy the share token from the generated URL. Its origin is the tunnel's
   `127.0.0.1:3001`; the public link is
   `https://hooklook.app/public-dashboards/<share-token>`. Open it in a private
   window: panels should load, and `https://hooklook.app/login` and
   `https://hooklook.app/api/dashboards/uid/hooklook-operator` must not reach
   Grafana.
4. Put the public link at the top of the repository README.

The share is Grafana runtime state in `hooklook-grafana`, so deployments keep
it. Pause or revoke it from the same drawer when the dashboard changes in a way
you have not reviewed, or when it must go offline. Revoking creates a new token
on the next share, so update the README link. Watch the query load the public
link generates on the operator dashboard's runtime panels.
