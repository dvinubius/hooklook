# hooklook

A small, self-hosted request bin written in Go. The backend now creates one cookie-associated bin per browser, captures arbitrary HTTP requests, and provides owner or invited-guest inspection APIs. The inspector page is still a placeholder; the Vue frontend is next.

## Intended v1 use

Hooklook is designed for small-scale integration testing. The current limits
and planned production limits differ; see the
[current architecture](docs/architecture.md) and
[production design notes](.agents/design-notes.md).

## Live request events

`GET /api/bins/{binCode}/events` opens an authorized Server-Sent Events stream. Owners send their cookie; guests append their invitation identifier as `?invite=...`. Each successfully persisted inbound request produces one
`request` event whose `data` is the same compact request summary returned by
the request-list endpoint. Deletion and clearing publish a `refresh` event. Events are intentionally ephemeral: clients must
refetch the list after reconnecting.

Each subscriber has a one-event buffer. If it cannot consume the next event,
hooklook closes that stream immediately; this prevents a slow browser from
blocking ingestion or accumulating unbounded work. Deleting a bin and graceful
server shutdown also close its open event streams.

## Project documentation

- [Plan](.agents/PROJECT_PLAN.md)
- [Current progress](.agents/PROGRESS.md)
- [Completed milestones](.agents/done-milestones.md)
- [Current architecture](docs/architecture.md)
- [Current HTTP API](docs/http-api.md)
- [Architecture decision records](docs/adr/)

## Local development

`PUBLIC_BASE_URL` is required to construct capture URLs. The server binds to
`127.0.0.1:8080`. `ADMIN_TOKEN` is required to start the server and protects
`GET /admin/bins` through a bearer token. Configuration comes from process
environment variables; `.env` is a shell
convenience, not an application configuration format.

```bash
set -a
source .env
set +a
export ADMIN_TOKEN=your-local-operator-secret
go run .           # start the server
# Open http://localhost:8080/ in a browser to create a bin
# Optional: go run . dev-bin prints a fixture capture URL
```

The `dev-bin` command creates a fixture directly in SQLite. Normal browser use starts at `/`, which creates or reuses a bin and redirects to its inspector page. The page currently shows a placeholder while the frontend is under review. See the [HTTP API](docs/http-api.md) for inspection and owner mutation routes.
The backend enforces 500 requests and 100 MB (100,000,000 bytes) of raw request
bodies in total per bin. Headers and metadata do not count. The Go-specific
body and header policy limits have been removed. Keep this build on loopback;
configure Caddy body, 32 KiB total-header, and rate limits before exposing it publicly.

## Capture smoke test

With the server running locally and a bin already created, exercise the current
Milestone 2 capture shapes with:

```bash
./scripts/exercise-capture.sh YOUR_BIN_CODE
```

Set `BASE_URL` to test a different instance, for example
`BASE_URL=http://localhost:8081 ./scripts/exercise-capture.sh YOUR_BIN_CODE`.
The script sends capture requests only; inspect the bin separately.

## Header redaction policy

Hooklook redacts common credential-bearing headers before storing a request.
Webhook signature headers are retained for debugging, so treat every bin as
sensitive. See [ADR 0002](docs/adr/0002-captured-header-redaction.md) for the
exact header list, repeated-header behavior, and security implications.
