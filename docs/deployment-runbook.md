# Deployment runbook

This runbook deploys the current Hooklook working tree to a prepared single
Docker host. It does not provision a VM, install Docker, configure DNS, or
change Caddy. Caddy is shared VPS infrastructure managed separately from
`/opt/caddy`.

## Prerequisites

- A reachable Linux VM with Docker, Docker Compose, and `curl` installed.
- SSH access for the deployment user (default: `root`).
- The Caddy-managed external Docker network `hooklook-edge` already exists.
- Port `127.0.0.1:8081` is available for Hooklook's loopback health check.
- For the public smoke check, `hooklook.app` resolves to the VM and Caddy has
  a valid route and certificate for it.
- An untracked local `.env.production` provides the required deployment
  values. Do not commit it:

  ```bash
  DEPLOY_HOST=your-vps-ip-or-hostname
  # DEPLOY_USER=root
  # DEPLOY_SSH_KEY=/path/to/private/key
  PUBLIC_BASE_URL=https://hooklook.app
  ADMIN_TOKEN=at-least-32-url-safe-random-characters
  # MAX_STORE=5000000000
  ```

`ADMIN_TOKEN` must have at least 32 URL-safe characters (`A-Z`, `a-z`, `0-9`,
`_`, and `-`). `PUBLIC_BASE_URL` must be an HTTPS origin with no path. The
default SQLite cap is 5,000,000,000 bytes; set `MAX_STORE` only to use another
positive byte count.

Before any remote changes, the deploy script verifies that the Docker data-root
filesystem has at least `2 × MAX_STORE + 2 GB` free. With the default cap this
is 12 GB, covering database growth, a backup staging copy, and operating
headroom. See [storage capacity and backups](storage-capacity.md) for why.

## Deploy

Load the deployment values without placing secrets in shell history:

```bash
set -a
source .env.production
set +a
./scripts/deploy.sh
```

The script's first action is always `go test -count=1 ./...`; it performs no
configuration validation, SSH connection, or source synchronization before
that test gate passes. It then:

1. Checks the remote Docker, Compose, `curl`, and `hooklook-edge` prerequisites.
2. Checks storage headroom before changing remote source or container state.
3. Synchronizes the application to `/opt/hooklook`, excluding Git metadata,
   local environment files, SQLite files, frontend dependencies/build output,
   and coverage artifacts. Because synchronization uses `--delete`, reserve
   that directory for Hooklook deployment files.
4. Writes `/opt/hooklook/.env` with mode `0600`, builds a candidate image, and
   recreates only the `hooklook` Compose service.
5. Polls `http://127.0.0.1:8081/health` for up to one minute, confirms the
   service is running, then verifies `https://hooklook.app/health`.

The deployment does not build, restart, reload, or otherwise alter Caddy or
zibs. It also preserves the `hooklook-data` named volume.

## Verify

The script completes only after both the local and public health checks pass.
For an independent check on the VM:

```bash
cd /opt/hooklook
docker compose ps
curl --fail http://127.0.0.1:8081/health
curl --fail https://hooklook.app/health
```

To inspect the protected effective storage limit without typing the token into
shell history, load the root-readable runtime environment first:

```bash
cd /opt/hooklook
set -a
source .env
set +a
curl --fail \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://127.0.0.1:8081/admin/storage
```

If the loopback health check passes but the public check fails, inspect the
shared ingress project at `/opt/caddy`. Do not rerun a Hooklook deployment as
an ingress repair.

## Diagnostics

On a failed candidate health or public smoke check, the deploy script prints
the Hooklook service state and its last 100 log lines, then restores the prior
application image when it has one. To investigate later:

```bash
cd /opt/hooklook
docker compose ps
docker compose logs --tail=100 hooklook
docker network inspect hooklook-edge
```

An unreachable `hooklook:8080` upstream or a missing shared network is an
ingress/container-connectivity issue; Caddy configuration remains in
`/opt/caddy`.

## Roll back

The script saves the previously running application image as
`hooklook:rollback-<timestamp>` before it builds the candidate. A failed
rollout restores that image automatically when one exists. To roll back a
deployment that completed successfully but must be reversed later:

```bash
cd /opt/hooklook
previous_image=$(cat .previous-image)
docker image tag "$previous_image" hooklook:latest
docker compose up -d --no-deps --force-recreate hooklook
curl --fail http://127.0.0.1:8081/health
curl --fail https://hooklook.app/health
```

The first-ever deployment has no previous image. In that case, check out a
known-good source revision locally and deploy it through the normal procedure.
Neither rollback path changes the `hooklook-data` volume. Do not use
`docker compose down -v` or remove that volume during a code rollback.
