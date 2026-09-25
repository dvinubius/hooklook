# Deployment security

Hooklook is a public request bin on a single Docker host. It deliberately
accepts untrusted webhook traffic and stores some of that traffic for an
authorized inspector. The deployment therefore separates public ingress,
private telemetry, operator access, and persistent data. This document
describes the implemented topology and its limits; the
[architecture](architecture.md) shows the full container diagram.

Public ingress is not in this repository. Caddy, its TLS certificates, public
ports, routes, and edge limits belong to
[hetzner-one](https://github.com/dvinubius/hetzner-one); its
[`Caddyfile`](https://github.com/dvinubius/hetzner-one/blob/main/Caddyfile) is
the source of truth for the limits described below.

## Reachability and trust boundaries

| Boundary | Members and exposure | Security purpose |
| --- | --- | --- |
| Internet to Caddy | Caddy serves `https://hooklook.app` and forwards the Hooklook hostname to port 8080 on `hooklook-edge`. | Caddy terminates TLS and applies public request rate, body-size, and header-size limits before traffic reaches Go. |
| `hooklook-edge` | Caddy and Hooklook share this Caddy-owned external Docker network. Hooklook also publishes its HTTP listener as `127.0.0.1:8081` for host checks. | Caddy can reach Hooklook without publishing the app on a public host interface. Caddy can reach both of Hooklook's listening ports, including 9092, through this bridge. The loopback port remains reachable by processes on the host. |
| `hooklook_metrics` | Internal Compose network shared by Hooklook and Prometheus. Hooklook listens on port 9092 inside its container; there is no host publication or Caddy route for `/metrics`. | Prometheus can reach both of Hooklook's listening ports through this bridge. Other application containers do not join it. Host administrators retain access to Docker networks. |
| `hooklook_observability` | Internal Compose network shared by Prometheus, Alloy, Loki, and Grafana. | Keeps telemetry APIs away from the ingress network. Prometheus bridges metrics into the private observability network for Grafana queries. |
| `hooklook_grafana-access` | Non-internal Compose bridge with Grafana as its only member; Grafana publishes `127.0.0.1:3001:3000`. | Allows Docker to create a working host-loopback publication without putting Grafana on `hooklook-edge`. Operators use an SSH tunnel and Grafana credentials. |

Compose fixes the project name to `hooklook`, which prefixes the three
Compose-owned network names above. `hooklook-edge` is external in the Compose
sense: Caddy's project, hetzner-one, creates and owns it. `external` describes network
ownership, while `internal` controls Docker's external routing behavior.
Neither setting is an application authorization rule. Containers on the same
bridge can initiate connections to one another; these networks do not provide
a directional firewall. In particular, Hooklook and Prometheus share a bridge,
and Prometheus also joins the observability bridge. Hooklook binds ports 8080
and 9092 to `0.0.0.0` inside its container, so both ports accept connections
from either network joined by that container. The `9092/tcp` entry in
`docker compose ps` is exposure metadata, not a host publication or a firewall
rule. The public cannot reach 9092 through Caddy's current routes, while
Caddy itself can reach that port on `hooklook-edge`. The reverse also holds:
Caddy's unpublished metrics listener, port 9180, is on `hooklook-edge`, so the
Hooklook container can reach it. It serves metrics only, not Caddy's admin API,
which stays on the Caddy container's loopback; see hetzner-one's
[topology](https://github.com/dvinubius/hetzner-one/blob/main/docs/topology.md).

Zibs is another application on the same VPS, also served by the shared Caddy
through its own `zibs-edge` network. It has its own Compose project,
networks, telemetry volumes, Grafana credentials, and dashboard. Hooklook does not attach to them. Both projects
still share the VPS and Docker daemon, so project separation does not protect
one from a compromised host or Docker administrator.

## Public request and stored-data controls

Caddy handles the public edge limits. Its capture route has a 10 MB
(10,000,000-byte) request-body limit and per-IP request rate limits. Together,
these offer a minimal guard against accidental bursts of heavy requests from
one IP; they are not effective DDoS protection. The shared HTTPS listener
retains `max_header_size 32KiB`. The Go application enforces bin access,
storage limits, and persistence rules after a request passes Caddy. The
`/b/{code}` capture endpoint is intentionally public to anyone who knows a
valid bin code. A bin code alone does not authorize reading its contents:
inspection needs the owner's cookie or an enabled invitation. Guest access is
read-only, and owner mutations require a matching `Origin`. Operator
`/admin/*` routes require the separate `ADMIN_TOKEN` bearer credential. See
[bin access](bin-access.md) for the exact authorization rules.

The owner secret is stored as a SHA-256 digest in SQLite and sent to the
browser in an `HttpOnly`, `SameSite=Lax` cookie marked `Secure` for the
production HTTPS origin. Inspection responses use `Cache-Control: no-store`
and `Referrer-Policy: no-referrer`. Those headers reduce accidental browser
retention or URL leakage; they do not make an owner cookie or invitation safe
to share.

Captured data is sensitive even when the sender meant it as test traffic.
Hooklook redacts common credential headers before storage, but preserves
request bodies, raw query strings, paths, and some webhook signature headers
for debugging. Authorized inspectors and anyone with access to the SQLite
volume or a backup can read the stored captures. Expiration removes inactive
bins after three days; per-bin count and body-byte limits and the SQLite
`MAX_STORE` cap bound ordinary growth. They are capacity and retention controls,
not substitutes for protecting the volume or backups. See
[header redaction](adr/0002-captured-header-redaction.md),
[bin lifecycle](bin-lifecycle.md), and [storage capacity](storage-capacity.md).

## Containers, telemetry, and privileged access

The application image runs as UID/GID 10001. Its root filesystem is read-only,
with a bounded temporary filesystem and a persistent `hooklook-data` volume.
The Compose service drops Linux capabilities and disallows privilege gains.
Prometheus, Loki, and Grafana also use read-only root filesystems, temporary
filesystems, dropped capabilities, and their own persistent volumes.
These settings reduce what a compromised process can change inside its
container; they do not override its network access or make the shared Docker
host a security boundary.

Alloy is the significant exception in authority. It reads Hooklook's Docker
stdout through `/var/run/docker.sock`, filtering discovery by the `hooklook`
Compose project and service labels before forwarding logs to Loki. That filter
limits the logs stored in Loki, **not** Alloy's Docker API permissions. A
`:ro` bind mount of the socket does not make API calls read-only. Alloy runs
as UID 0 to open the root-owned socket and with GID 473 to write its state
volume while its Linux capabilities are dropped. Access to the Docker daemon
can affect other containers on the VPS, including zibs; a Docker socket proxy
or a different log collection path would be a separate security improvement.

Hooklook emits structured logs and bounded-label metrics without captured
bodies, raw paths, query strings, cookies, invitation IDs, or credential
headers. The telemetry pipeline retains application logs in Loki for seven
days and metrics in Prometheus for fourteen days. Grafana disables anonymous
access and signup and uses Hooklook-specific credentials. None of these
services has a public Caddy route. See the
[observability runbook](observability-runbook.md) for validation and access.

## Operator credentials, deployment, and recovery

GitHub Actions deploys over SSH as `hooklook-deploy`, an account used only
for deployment, with a dedicated ed25519 key marked `restrict` in its
`authorized_keys`. Docker group membership makes that account effectively
root on the host, so the private key lives only in the `production`
environment secret, whose deployments are limited to `main`, and on the
operator's workstation. The workflow checks the VPS host key against the
verified `DEPLOY_KNOWN_HOSTS` line and never scans it. For a full deployment,
the VPS receives the run's short-lived `GITHUB_TOKEN` on stdin, logs in to
GHCR with a temporary Docker config, and deletes it after the pull; the VPS
holds no Git credential or PAT and never clones the repository.

Runtime secrets stay on the VPS in `.env` and `.env.observability` (mode
`0600`); deployments never write them. Compose environment variables remain
visible to host administrators and anyone with Docker daemon access.
Dashboard-only deployment installs one JSON file; observability-only
deployment recreates Prometheus, Alloy, Loki, and Grafana but not Hooklook.
Neither touches zibs or Caddy.

Because the runners' addresses change, SSH is reachable from any source.
`sshd` accepts keys only (no password or keyboard-interactive login, root by
key only), limits authentication attempts and unauthenticated connection
time, and fail2ban bans addresses that repeatedly fail. See the
[deployment runbook](deployment-runbook.md#ssh-reachability-and-hardening).

The live SQLite database resides in `hooklook-data`. The backup command
creates a consistent snapshot and a SHA-256 sidecar outside that volume, with
mode `0600` files. Treat both the live volume and snapshots as captured
production data. The current backup workflow creates a local staging pair;
automated off-host R2 storage is planned but not yet implemented. Host loss can
therefore still mean data loss unless an operator has transferred and verified
a backup. See the [backup runbook](database-backup-runbook.md).

## Checks after a topology change

- Run the [public verification](production-verification-runbook.md) to check
  HTTPS, authorization, body/header limits, and exposure from outside the VPS.
- Run the [telemetry smoke test](observability-runbook.md) to confirm the live
  Grafana loopback binding, Hooklook readiness, scrape, log delivery,
  datasources, and dashboard provisioning.
- Inspect Compose network membership and published ports on the VPS after
  changing Compose or Caddy. Verify `/metrics`, Loki, Prometheus, and Grafana
  have no public route or public host binding, and recheck zibs independently.
- Protect SSH keys, remote environment files, the Docker socket, the SQLite
  volume, and backup snapshots as the remaining high-value access paths.
