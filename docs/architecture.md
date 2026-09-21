# Current architecture

Hooklook is one Go `net/http` service using SQLite through
`github.com/mattn/go-sqlite3`. It validates `PUBLIC_BASE_URL` and `ADMIN_TOKEN`,
opens `hooklook.db`, creates a fresh development schema, and listens on
`127.0.0.1:8080`. It shuts down on SIGINT/SIGTERM, closing event streams before
graceful HTTP shutdown. Caddy is planned but not yet configured in this
repository.

A home visit creates or resolves one cookie-associated bin and redirects to
`/bins/{code}`. Codes are readable adjective-noun-number addresses. A
cryptographically random ownership secret is stored only as a SHA-256 digest in
SQLite and sent in an `HttpOnly` browser cookie. Each bin has a distinct
reusable invitation identifier. The owner can enable guest read access;
disabling it revokes API access and closes the guests' streams, leaving the
owner's own stream connected. Authorized page
requests are answered with the Vue application itself.

The frontend build is embedded in the binary with `go:embed`, so the binary is
the whole deployment. `frontend/dist` must therefore exist at Go build time;
the committed placeholder keeps a fresh checkout compilable, and a binary built
without real assets reports the missing build on its page routes rather than
serving an empty document. Both `/bins/{code}` and the capture-reported detail
URL `/bins/{code}/requests/{id}` serve the same document after the same
authorization, and the application resolves the request id itself. Because the
document references its assets by absolute path, direct navigation and reloads
work at either depth. Asset routes skip bin authorization; page routes keep
`Cache-Control: no-store` and `Referrer-Policy: no-referrer`. Under
`FRONTEND_DEV` the page routes serve a shell pointing at Vite's module graph
instead, so development keeps one browser origin without bypassing
authorization.

Inside that document the application treats the server as the only authority on
access: it fetches `GET /api/bins/{code}` before anything private renders, keeps
the request list reconciled against SQLite rather than against the stream, and
renders captured bytes as text and never as markup. The
[frontend](frontend.md) describes those mechanisms.

The list, detail, metadata, and SSE routes check owner or guest authorization.
Destructive routes require the owner cookie and same-origin `Origin` header.
Detail sends the stored raw body as JSON base64 so binary data remains faithful.
Deleting requests adjusts `total_body_bytes` in the same SQLite transaction.
There is no bin replacement endpoint. SQLite foreign keys are enabled.

Public `/b/{code}` capture remains open to anyone with the code. `requests.go`
redacts credential-like header names and preserves method, path suffix, raw
query, other headers, raw body, content type, and receipt time. `store.go`
atomically checks 500-request and 100 MB raw-body budgets before insertion. SSE
publishes compact summaries after commit. A `refresh` event follows deletion or
clearing. SSE is ephemeral and clients must refetch on reconnect. Slow
subscribers are disconnected rather than blocking ingestion.

Bin expiry remains seven days from creation. Expiry renewal, cleanup, global
storage cap, backups, metrics, and Caddy policy belong to the later milestones.
No Go-specific body or header policy limit is applied; the service must stay on
loopback until Caddy enforces those limits.

See the [HTTP API](http-api.md), [frontend](frontend.md),
[project plan](../.agents/PROJECT_PLAN.md), and
[progress](../.agents/PROGRESS.md).
