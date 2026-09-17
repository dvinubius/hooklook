# Current HTTP API

This is the implemented API at the end of milestone 5. The
[production v1 plan](../.agents/PROJECT_PLAN.md) replaces token and admin
routes with cookie-associated bins and adds inspection/deletion endpoints.
Responses described here are from the Go service; a future Caddy edge may
reject requests before they reach Go.

The service listens on `:8080`. JSON responses use `Content-Type:
application/json`. Errors from handlers use plain text via `http.Error`.
All examples below use `http://localhost:8080`; set `PUBLIC_BASE_URL` to
the corresponding public origin when running the server.

| Method and path | Current behavior |
| --- | --- |
| `GET /health` | `200` with `OK, I'm healthy`. |
| `POST /api/bins` | Requires `Authorization: Bearer <creation-token>`. Consumes one use and creates a bin. Returns `201` with `{"code":"…","url":"<PUBLIC_BASE_URL>/b/…"}`. Missing, invalid, exhausted, or revoked tokens return `401` and `WWW-Authenticate: Bearer`. |
| Any method `/b/{code}` or `/b/{code}/{path...}` | Captures the request. Returns `201` with `{"id":"…","url":"<PUBLIC_BASE_URL>/bins/{code}/requests/{id}"}`. The returned detail URL is a placeholder: no matching route exists yet. A missing/expired bin returns `404`, a body over the Go limit returns `413`, a full bin returns `507`, and an internal failure returns `500`. |
| `GET /api/bins/{code}/requests` | Returns `200` and a JSON array of summaries, ordered by increasing request ID. A missing bin returns `404`. No sort/filter query parameters are implemented. |
| `GET /api/bins/{code}/events` | Returns `200` and `text/event-stream` for an existing bin. Each persisted capture sends `event: request` with a JSON summary in `data:`. A missing bin returns `404`; a hub closing during startup can return `503`. No replay or event IDs. |
| `GET /admin/bins` | Operator bearer token required. Returns bin summaries including request counts. |
| `DELETE /admin/bins/{code}` | Operator bearer token required. Returns `204` or `404`; closes its SSE streams after deletion. |
| `POST /admin/tokens` | Operator bearer token required. Accepts `{"label":"…","maxUses":N}`, where label is 1–100 characters and N is 1–25. Returns `201` with token metadata and the one-time plaintext `token`. Invalid input returns `400`. |
| `GET /admin/tokens` | Operator bearer token required. Returns token metadata without plaintext tokens. |
| `DELETE /admin/tokens/{id}` | Operator bearer token required. Revokes an active token; returns `204` or `404`. |

All `/admin/*` routes return `401` with `WWW-Authenticate: Bearer` when
the operator token is absent or invalid. `POST /api/bins` also returns
`500` if insertion fails. An unknown route returns Go's `404`.

## Captured request representation

The list response and SSE `request` event use the same summary:

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

`bodySizeKiB` is the body length rounded up to whole KiB; `headerCount`
counts distinct stored header names. Bodies and full headers are stored in
SQLite but have no public detail endpoint yet. Common credential header values
are replaced with `[REDACTED]` before storage; see
[ADR 0002](adr/0002-captured-header-redaction.md).

The optional suffix of a capture URL becomes `path`; capture at exactly
`/b/{code}` has an empty path. The raw query is preserved as received.
Inbound methods are not restricted to the usual webhook verbs.
