# Current architecture

This describes the implementation after the obsolete-functionality cleanup.
The [project plan](../.agents/PROJECT_PLAN.md) describes the production v1
destination; cookie-associated bins and the inspection UI are next.

Hooklook is one Go `net/http` process with SQLite through
`github.com/mattn/go-sqlite3`. `main.go` validates `PUBLIC_BASE_URL`,
opens `hooklook.db`, migrates the schema, and listens on
`127.0.0.1:8080` by default. It shuts down on SIGINT/SIGTERM, closing event
streams before graceful HTTP shutdown. A separate Caddy deployment is planned
but is not configured in this repository. The loopback listener keeps this
transitional build private until Caddy ingress limits are installed.

The router serves health, capture, request-list, SSE, and a read-only operator
bin-list route. There are no creation-token or other admin HTTP routes.
`GET /admin/bins` requires the configured `ADMIN_TOKEN` bearer secret.
Until the home page creates bins, local
development can create a fixture with `go run . dev-bin`. That command uses
the same store but does not add an HTTP creation path.

For inbound `/b/{code}` requests, `requests.go` copies headers while
redacting credential-like names, reads the body, and records method, optional
path suffix, raw query, content type, body bytes, and UTC receipt time.
`store.go` checks the bin's unexpired state, 500-request count, and
100 MB (100,000,000 bytes) total raw-body allowance in a transaction before inserting.
The handler publishes a compact summary through `events.go` only after
commit. There is currently no Go-specific body or header policy limit; the
future Caddy edge must apply a body limit and a 32 KiB total request-header
limit before public exposure.

The fresh development schema contains `bins` and `requests`; it does not
migrate or backfill the earlier development database. Each bin has a
`total_body_bytes` counter. Only raw request-body bytes count toward the
100 MB bin allowance. Headers, paths, queries, and other metadata do not.
Bin expiry remains seven days after creation and has no cleanup worker yet.

`EventHub` is process-local and keyed by bin code. Each SSE subscriber has a
single buffered summary event. A slow subscriber is closed instead of blocking
ingestion; clients reconnect and refetch persisted requests. Disconnect and
server shutdown release subscriptions. Events are not replayed.

The Go server retains 5-second header, 15-second read, and 60-second idle
timeouts. There is no home-page UI, cookie ownership, request-detail or
deletion route, storage-wide cap, backup flow, cleanup worker, or metrics.
`/health` is a static liveness response. Request lists are returned in
insertion order without server-side sort or filters.

See the [current HTTP API](http-api.md) for routes and response shapes.
