# Production verification runbook

Use this runbook after a Hooklook or shared-Caddy deployment. It verifies the
public contract through `https://hooklook.app` with protocol-level requests;
it does not use browser automation.

The public verifier creates one temporary Hooklook bin and test captures. It
does not restart containers, reload Caddy, or alter zibs. Run it from a trusted
workstation with the production operator token already exported:

```bash
./scripts/verify-public.sh
```

It checks HTTPS health, the home redirect and owner cookie, the embedded
frontend asset, capture/list/detail authorization, sharing and revocation,
admin bearer authorization, the 10 MB (10,000,000-byte) capture-body boundary
with fixed-length and chunked requests, and the listener-wide header boundary.
Caddy is configured with
`max_header_size 32KiB`; Go's HTTP/1.1 parser adds a 4 KiB buffer allowance,
so the observed rejection boundary is approximately 36 KiB. The verifier uses
30 KiB as its accepted case and 40 KiB as its rejected case. The script leaves
its temporary bin to expire normally. One accepted boundary capture stores a
10 MB body, so allow for that temporary SQLite usage on each run. Run this
version only after the Caddy policy has been deployed.

## Intentional rate-limit checks

The rate-limiter test deliberately consumes the current public source IP's
creation and capture allowance. Run it separately after ordinary verification,
and avoid a NAT, VPN, or proxy shared with active users:

```bash
VERIFY_RATE_LIMITS=1 \
./scripts/verify-public.sh
```

It expects `429` plus `Retry-After` for the eleventh home request in its own
one-minute window and for the twenty-first capture in a 20-second window. It also proves that an
ordinary authorized inspector API request remains available after each limit.
The script waits 21 seconds before its capture-burst test and can take more
than a minute because it verifies that an SSE stream survives idle time.

## Capacity responses

Do not fill a production bin or SQLite store merely to trigger `507`. The
stable `bin_full` and `store_full` headers/statuses are exercised by the focused
Go persistence tests against temporary SQLite databases. Capacity incidents in
production should be observed and handled as real operational events, not
manufactured during this runbook.

## Controlled graceful-shutdown check

This check creates a brief Hooklook interruption; schedule it deliberately.
On the VPS, stop only Hooklook, inspect the persisted database with the
application image, then start the same service again:

```bash
cd /opt/hooklook
./scripts/compose.sh stop -t 15 hooklook
./scripts/compose.sh logs --since=2m hooklook
./scripts/compose.sh run --rm --no-deps --user 0 \
  hooklook integrity-check /data/hooklook.db
./scripts/compose.sh up -d --no-deps --no-build hooklook
curl --fail http://127.0.0.1:8081/health
curl --fail https://hooklook.app/health
```

The stop log should include the orderly shutdown message, and the integrity
command must print `ok`. To observe an SSE close specifically, open an
authorized `curl --no-buffer` SSE connection before stopping the service; it
should terminate cleanly when the service stops.

## Exposure checks

From the VPS, confirm Hooklook has only its loopback publication and is on the
shared Caddy network:

```bash
cd /opt/hooklook
./scripts/compose.sh ps
docker network inspect hooklook-edge
```

From a machine outside the VPS, `http://<VPS-IP>:8081/health` must not be
reachable; use `https://hooklook.app/health` instead.

If a loopback check passes but a public check fails, inspect `/opt/caddy` as an
ingress incident. Do not rerun a Hooklook deploy as a Caddy repair.
