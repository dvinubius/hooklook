# Current architecture

This document describes the implementation at the end of milestone 5, before
the cookie-based production v1 changes in the [project plan](../.agents/PROJECT_PLAN.md).
It is a reference for migration work, not the target architecture.

## Process and data flow

Hooklook is one Go `net/http` process. `main.go` reads configuration, opens
`hooklook.db`, runs the schema creation in `db.go`, starts the HTTP server
on `:8080`, and shuts it down on SIGINT/SIGTERM. It closes all event streams
before graceful HTTP shutdown. The database is SQLite via
`github.com/mattn/go-sqlite3`; there is no separate payload filesystem.
Deployment behind Caddy is planned but is not configured in this repository.

The HTTP router in `main.go` sends public API and capture requests to
`handlers_public.go` and operator routes to `handlers_admin.go`. Creating a
bin requires a creation-token bearer credential. The middleware consumes one
token use before the handler inserts the bin, so a later create failure does
not restore that use. Operator routes require `ADMIN_TOKEN` as a separate
bearer credential.

For an inbound `/b/{code}` request, `requests.go` copies headers while
redacting credential-like names, reads the body with a size limit, and records
the method, optional path suffix, raw query, content type, body bytes, and UTC
receipt time. `store.go` atomically checks the bin's unexpired state, the
500-request count, and the 10 MiB rounded-up body-size allowance before
inserting the capture. It returns a SQLite-generated integer request ID
represented as a string in JSON. The handler publishes a compact summary to
`events.go` only after the transaction commits.

The database has `bins`, `requests`, and `creation_tokens` tables. A bin
has a random 22-character code, creation and expiration times, and a stored
body-KiB counter. The current expiration time is seven days after creation;
the insert path rejects expired bins, but no automatic cleanup worker exists.
Requests store headers as JSON and raw bodies as BLOBs. The schema declares a
foreign key with cascade deletion; the migration executes
`PRAGMA foreign_keys = ON` during setup.

## Live events

`EventHub` is process-local and keyed by bin code. Each SSE subscriber has
one buffered summary event. Publishing does not block on a browser: a slow
subscriber is closed, and the client must reconnect and refetch persisted
requests. Disconnect, operator bin deletion, and server shutdown release
subscriptions. Events are never replayed or stored separately from captures.

## Current boundaries

- Go reads at most `MAX_REQUEST_BODY_BYTES + 1` bytes per capture; the
  default limit is 256 KiB. `newHTTPServer` sets `MaxHeaderBytes` to 32 KiB,
  plus 5-second header, 15-second read, and 60-second idle timeouts.
- `PUBLIC_BASE_URL` and `ADMIN_TOKEN` are required; `MAX_REQUEST_BODY_BYTES`
  is optional. The database path is currently the fixed relative
  `hooklook.db`.
- There is no home-page UI, cookie ownership, request-detail route, request
  deletion route, storage-wide cap, backup flow, cleanup worker, or metrics.
  The current `/health` is a static liveness response.
- Request summaries are returned in insertion order. The planned sort and
  filters are not yet implemented in the API.

See the [current HTTP API](http-api.md) for exact routes and response shapes.
