# Current HTTP API

This is the contract the Inspection UI ("inspector") is built on; the
[frontend](frontend.md) describes how it consumes it. The page routes serve the
built Vue application after authorization. The server binds to `127.0.0.1:8080`
by default.

| Method and path | Behavior |
| --- | --- |
| `GET /` | Resolve the browser's cookie-associated bin, creating one when needed, then `303` to `/bins/{code}`. |
| `GET /bins/{code}` | Authorize owner cookie or `?invite={identifier}` while sharing is enabled, then serve the application document. Unauthorized visitors are `303`-redirected to their own bin, resolved or created, before any request data loads. |
| `GET /bins/{code}/requests/{id}` | The detail URL a capture reports. Same authorization and same document as the bin page; the request id is resolved inside the application, so an unknown id is not a server error. A guest's `?invite=` is preserved in the URL. |
| `GET /assets/{path...}`, `GET /fonts/{path...}` | The embedded frontend build. Deliberately outside bin authorization: no captured data, identical for every visitor, and required by a page the server has already handed over. Hashed JavaScript and CSS are `immutable`; fonts get an ordinary lifetime. Unknown files: `404`. |
| Any method `/b/{code}` or `/b/{code}/{path...}` | Capture an HTTP request and return `201` with its ID and the UI detail URL that opens it. Missing or expired bin: `404`; full bin: `507`. |
| `GET /api/bins/{code}` | Authorized bin metadata. Owners also receive `inviteId` and sharing state. |
| `GET /api/bins/{code}/requests` | Authorized body-free request summaries in ascending ID order. |
| `GET /api/bins/{code}/requests/{id}` | Authorized full detail, including redacted stored headers and `rawBody` encoded as base64 by JSON. A request that is not in the bin is `404`, which the UI reports as a stale selection rather than as revoked access. |
| `GET /api/bins/{code}/events` | Authorized SSE. `request` events contain summaries. `refresh` events tell clients to refetch after deletion or clearing. Reconnect and refetch after a stream closes. |
| `DELETE /api/bins/{code}/requests/{id}` | Owner only. Delete one request and reclaim its exact raw-body bytes. |
| `DELETE /api/bins/{code}/requests` | Owner only. Clear requests while retaining the bin address and invitation. |
| `PUT /api/bins/{code}/sharing` | Owner only. JSON body `{"enabled":true}` or `{"enabled":false}`; disabling sharing closes current SSE streams. Re-enabling uses the same invitation. |
| `POST /api/bins/{code}/replace` | Owner only. Atomically replace the bin, set a new cookie, and `303` to its page. Old capture and invitation links stop working. |
| `GET /admin/bins` | Operator-only list. Requires `Authorization: Bearer <ADMIN_TOKEN>`. |
| `GET /health` | Static liveness response. |

For authorized GET endpoints, owners use the `hooklook_owner` cookie and guests
provide `?invite={identifier}` on **each** API and SSE request. The cookie is
`HttpOnly`, `SameSite=Lax`, `Secure` when `PUBLIC_BASE_URL` uses HTTPS, and
expires with the bin. A bin code alone does not permit inspection. Owner
mutation requests require the cookie and an `Origin` header matching
`PUBLIC_BASE_URL`. Responses involving access state use
`Cache-Control: no-store` and `Referrer-Policy: no-referrer`.

New codes have an adjective-noun-eight-digit format. Bins currently expire seven
days after creation; renewal and cleanup are in the next milestone. Each bin
accepts at most 500 captures and 100 MB (100,000,000 bytes) of raw request
bodies. Headers and metadata do not count. Common credential headers are
redacted before storage. Public Caddy body, total-header, and rate limits are
still required before internet exposure.
