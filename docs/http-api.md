# Current HTTP API

This is the API after removing creation tokens and obsolete admin routes. The
[production v1 plan](../.agents/PROJECT_PLAN.md) adds a cookie-associated home
page, detail and deletion endpoints, and Caddy edge limits.

The Go service listens on `127.0.0.1:8080` by default. JSON responses use
`Content-Type: application/json`; handler errors use plain text. The server
requires `ADMIN_TOKEN` as an operator bearer secret. No HTTP
route currently creates a bin. For local testing, run `go run . dev-bin` to
print a new capture URL before starting the server.

| Method and path | Current behavior |
| --- | --- |
| `GET /health` | `200` with `OK, I'm healthy`. |
| Any method `/b/{code}` or `/b/{code}/{path...}` | Captures the request. Returns `201` with `{"id":"…","url":"<PUBLIC_BASE_URL>/bins/{code}/requests/{id}"}`. The detail URL is a placeholder: no matching route exists yet. Missing/expired bin: `404`; full bin: `507`; internal failure: `500`. |
| `GET /api/bins/{code}/requests` | `200` with a JSON array of summaries, ordered by increasing request ID. Missing bin: `404`. Sort/filter query parameters are not yet implemented. |
| `GET /api/bins/{code}/events` | `200` with `text/event-stream` for an existing bin. Each persisted capture sends `event: request` and a JSON summary in `data:`. Missing bin: `404`; hub closing during startup: `503`. No replay or event IDs. |
| `GET /admin/bins` | Requires `Authorization: Bearer <ADMIN_TOKEN>`. Returns `200` with bin summaries containing `code`, `createdAt`, `expiresAt`, `totalBodyBytes`, and `requestCount`. Missing or invalid bearer token: `401` with `WWW-Authenticate: Bearer`. |

`POST /api/bins` and all other `/admin/*` routes have been removed and return
`404`. The Go handler no longer returns `413` for body size and does not
set a custom header-size limit. Public deployment must apply its body-size
limit and a 32 KiB total-header-size limit at Caddy before proxying traffic.
The Go service enforces a per-bin maximum of 500 captures and 100 MB
(100,000,000 bytes) of raw request bodies in total. Headers and metadata do
not count toward that bin allowance.

## Request summary

The list response and SSE `request` event use the same shape:

```json
{
  "id": "1",
  "method": "POST",
  "path": "/github/events",
  "rawQuery": "source=example",
  "receivedAt": "2026-09-17T12:00:00Z",
  "contentType": "application/json",
  "bodySizeKiB": 1,
  "headerCount": 2
}
```

`bodySizeKiB` is body length rounded up to whole KiB; `headerCount` counts
distinct stored header names. Bodies and full headers are in SQLite but have
no detail endpoint yet. Common credential header values are replaced with
`[REDACTED]` before storage; see [ADR 0002](adr/0002-captured-header-redaction.md).
The optional capture URL suffix becomes `path`; capture at exactly
`/b/{code}` has an empty path. The raw query is preserved as received, and
inbound methods are unrestricted.
