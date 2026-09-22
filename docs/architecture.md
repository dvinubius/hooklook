# Current architecture

## Service and persistence

Hooklook is one Go `net/http` service backed by SQLite through
`github.com/mattn/go-sqlite3`. SQLite is the source of truth for bins and
captured requests; foreign keys cascade request deletion when a bin is removed.
The service keeps one database connection, configures SQLite's maximum page
count from `MAX_STORE`, and runs an application-owned expiration cleanup worker.

The process validates `PUBLIC_BASE_URL` and `ADMIN_TOKEN`, opens `hooklook.db`,
initializes the development schema, and listens on `127.0.0.1:8080`. SIGINT or
SIGTERM stops the cleanup worker, closes event streams, and gracefully shuts
down HTTP. The process also checks SQLite once a second. If the open database
becomes unusable, or expiration cleanup encounters a database error, the same
graceful shutdown runs and the process exits with an error. A separate `backup`
command uses SQLite `VACUUM INTO` to copy the database consistently while the
service is running. See [bin lifecycle](bin-lifecycle.md)
for retention and cleanup, and [storage capacity](storage-capacity.md) for
limits, backup, and restore. [Database behavior](db.md) describes the schema,
transactions, connection setup, and fatal database-failure policy.

## HTTP and frontend

The Go service handles the public capture route, authorized inspection routes,
operator route, and frontend assets. The frontend build is embedded with
`go:embed`, making the binary the deployment unit. `frontend/dist` must exist
when Go compiles; the committed placeholder permits a fresh checkout to build,
but page routes report a missing frontend build until real assets are compiled.

Both `/bins/{code}` and `/bins/{code}/requests/{id}` serve the same Vue
document after server authorization. The Vue application resolves the selected
request and fetches bin data through the API. Absolute asset paths make direct
navigation to either page route work. Asset routes are public because they
contain no captured data. With `FRONTEND_DEV`, page routes instead serve a shell
pointing to Vite while Go retains page authorization. See [bin access](bin-access.md)
for ownership and sharing, [frontend](frontend.md) for the client mechanisms,
and the [HTTP API](http-api.md) for routes and responses.

## Capture and live updates

`requests.go` parses inbound captures and redacts credential-like headers
before persistence. `store.go` writes each accepted capture and its bin counters
in a SQLite transaction. After commit, an in-memory event hub sends a compact
summary to subscribers of that bin. Delete and clear operations publish a
refresh signal. SSE is ephemeral: reconnecting clients fetch the persisted list
from SQLite, and slow subscribers are disconnected rather than delaying writes.
The [bin lifecycle](bin-lifecycle.md) describes expiration; [storage capacity](storage-capacity.md)
describes capture limits.

## Ingress boundary

The service listens on loopback. Caddy is intended to terminate TLS and apply
public rate, request-body, and total-header limits, but that proxy configuration
is not yet in this repository. Go currently applies no separate body or header
policy limit. The service should remain on loopback until the Caddy limits are
configured and verified.

See the [project plan](../.agents/PROJECT_PLAN.md) and
[progress](../.agents/PROGRESS.md) for upcoming work.
