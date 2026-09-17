# hooklook

A small, self-hosted request bin written in Go. The current backend lets
creation-token holders create temporary endpoints, send arbitrary HTTP
requests, and receive live capture events. The
[production v1 plan](.agents/PROJECT_PLAN.md) replaces creation tokens with
one cookie-associated bin per browser and adds the inspection UI.

## Intended v1 use

Hooklook is designed for small-scale integration testing. The current limits
and planned production limits differ; see the
[current architecture](docs/architecture.md) and
[production design notes](.agents/design-notes.md).

## Live request events

`GET /api/bins/{binCode}/events` opens a Server-Sent Events stream for one
existing bin. Each successfully persisted inbound request produces one
`request` event whose `data` is the same compact request summary returned by
the request-list endpoint. Events are intentionally ephemeral: clients must
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

## Local configuration

`PUBLIC_BASE_URL` is required to create bins because it is used to construct
their inbound URLs. For local development:

```bash
set -a
source .env
set +a
go run .
```

Hooklook reads process environment variables only; `.env` is a local shell and
deployment convenience, not an application configuration format.

### Current request-body limit

`MAX_REQUEST_BODY_BYTES` sets the largest request body the current Go backend
will capture. It defaults to `262144` (256 KiB). Oversized bodies and headers
are rejected before storage. The production plan moves these ingress limits to
Caddy.
