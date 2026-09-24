# Current HTTP API

This is the contract the Inspection UI ("inspector") is built on; the
[frontend](frontend/frontend.md) describes how it consumes it. The page routes serve the
built Vue application after authorization. The server binds to `127.0.0.1:8080`
by default.

| Method and path | Behavior |
| --- | --- |
| `GET /` | Resolve the browser's cookie-associated bin, creating one when needed, then `303` to `/bins/{code}`. At storage capacity, return `507` without a cookie, with `X-Hooklook-Error: store_full` and `data-hooklook-startup="store_full"` on the app document's `<html>` element. A standalone explanation is the fallback if frontend assets are absent. |
| `GET /bins/{code}` | Authorize owner cookie or `?invite={identifier}` while sharing is enabled, then serve the application document. An unavailable regular URL returns a marked `404` expiration page. An unavailable URL with a nonempty `invite` parameter returns a marked `404` shared-bin page. This follows the link shape whether the target is missing, expired, invalid, or revoked. Both pages require an explicit **Create New Bin** action before visiting `/`. |
| `GET /bins/{code}/requests/{id}` | The detail URL a capture reports. It has the same authorization and unavailable-bin behavior as the bin page. For an authorized active bin, the request id is resolved inside the application, so an unknown request id is not a server error. A guest's `?invite=` is preserved in the URL. |
| `GET /bins/{code}/`, `GET /bins/{code}/requests/{id}/` | `301` to the same address without the trailing slash, query string kept. Nothing is looked up; the canonical URL is authorized as usual. |
| `GET /bins/{code}/requests`, `GET /bins/{code}/requests/` | `301` to `/bins/{code}`, query string kept. Nothing is looked up; the bin page is authorized as usual. |
| `GET /assets/{path...}`, `GET /fonts/{path...}` | The embedded frontend build. Deliberately outside bin authorization: no captured data, identical for every visitor, and required by a page the server has already handed over. Hashed JavaScript and CSS are `immutable`; fonts get an ordinary lifetime. Unknown files: `404`. |
| Any method `/b/{code}` or `/b/{code}/{path...}` | Capture an HTTP request and return `201` with its ID and the UI detail URL that opens it. Missing or expired bin: `404`. A full bin returns `507` with `X-Hooklook-Error: bin_full` and `bin storage limit reached`; a full global store returns `507` with `X-Hooklook-Error: store_full` and `global storage limit reached`. The bin limit takes precedence when both are full. |
| `GET /api/bins/{code}` | Authorized bin metadata, per-bin `capacity`, and global `storeCapacity` for owners and invited guests. Owners also receive `inviteId` and sharing state. |
| `GET /api/bins/{code}/requests` | Authorized body-free request summaries in ascending ID order. |
| `GET /api/bins/{code}/requests/{id}` | Authorized full detail, including redacted stored headers and `rawBody` encoded as base64 by JSON. A request that is not in the bin is `404`, which the UI reports as a stale selection rather than as revoked access. |
| `GET /api/bins/{code}/events` | Authorized SSE. The stream opens with a `: connected` comment. `request` events contain summaries. `refresh` events tell clients to refetch after deletion or clearing. Reconnect and refetch after a stream closes. |
| `DELETE /api/bins/{code}/requests/{id}` | Owner only. Delete one request and reclaim its exact raw-body bytes. |
| `DELETE /api/bins/{code}/requests` | Owner only. Clear requests while retaining the bin address and invitation. |
| `PUT /api/bins/{code}/sharing` | Owner only. JSON body `{"enabled":true}` or `{"enabled":false}`; disabling sharing closes the guests' SSE streams and leaves the owner's open. Re-enabling uses the same invitation. |
| `GET /admin/bins` | Operator-only list. Requires `Authorization: Bearer <ADMIN_TOKEN>`. |
| `GET /admin/storage` | Operator-only global SQLite capacity statistics. Requires `Authorization: Bearer <ADMIN_TOKEN>`. |
| `GET /health` | Static liveness response. |

Unavailable page documents carry their state in both `X-Hooklook-Error` and
the `<html>` element's `data-hooklook-startup` attribute. An unavailable URL
with a nonempty `invite` parameter uses `shared_bin_unavailable`; an unavailable
regular URL uses `bin_expired`. This classification depends only on the link
shape, so it also applies to missing and expired targets. Both responses are
`404`, set no replacement cookie, and make no inspection API call. Authorized
API requests that later lose access use the same rule and error header,
allowing an already-open page to show the matching empty state.

There is no client-address field in any response. The UI shows a **client ip**
on a request's detail, but it reads that out of the stored `X-Forwarded-For`
header in this endpoint's `headers`, and it reports the *last* hop: Caddy
appends the address it accepted the connection from, so the first hop is only
what the caller claimed. Nothing about the address is captured or stored
separately, and the trust rule assumes Caddy is the only public ingress — see
[frontend](frontend/frontend.md#client-address).

The `capacity` object in `GET /api/bins/{code}` contains `requestCount`,
`requestLimit`, `bodyBytesUsed`, `bodyBytesLimit`, `requestsFull`,
`bodyBytesFull`, and `full`. The limits are currently 500 requests and
100,000,000 raw body bytes. `full` is true when either limit has been reached;
the two specific flags identify which one. A body-byte-full bin can still
accept an empty-body capture while request slots remain. A capture can also
return `bin_full` before either flag becomes true when its body exceeds the
remaining byte allowance. Fetch bin metadata again after an SSE `request` or
`refresh` event to get current capacity; SSE summaries do not carry it. This
object reports the bin's limits. The sibling `storeCapacity` object reports the
global SQLite main-file budget: `maxBytes` (the configured cap rounded down to
whole pages), `databaseBytes` (current file pages, including reusable pages),
`reusableBytes` (free pages inside that file), `availableBytes` (unused capacity
plus reusable pages), and `full`. The `full` flag is true when fewer than two
pages are available, the same minimum used by the write precheck for a new bin
or a small capture. Larger captures can still return `store_full` while this
flag is false. Filesystem free space is not included. Refetch bin metadata
after SSE events to update either capacity object.

`GET /admin/storage` returns the same `maxBytes`, `databaseBytes`,
`reusableBytes`, `availableBytes`, and `full` fields as `storeCapacity`. It also
returns `usedBytes`, calculated as `databaseBytes - reusableBytes`, and
`usedPercent`, calculated against `maxBytes`. `databaseBytes` is the physical
main-file allocation; `usedBytes` is the portion occupied by live SQLite pages.
The percentage can exceed 100 when an existing database is larger than the
configured limit. Filesystem free space and journal or temporary files are not
included.

For authorized GET endpoints, owners use the `hooklook_owner` cookie and guests
provide `?invite={identifier}` on **each** API and SSE request. The cookie is
`HttpOnly`, `SameSite=Lax`, `Secure` when `PUBLIC_BASE_URL` uses HTTPS, and
expires with the bin. A bin code alone does not permit inspection. Owner
mutation requests require the cookie and an `Origin` header matching
`PUBLIC_BASE_URL`. Responses involving access state use
`Cache-Control: no-store` and `Referrer-Policy: no-referrer`.

New codes have an adjective-noun-two-digit format. Bins expire after three days
of inactivity. Successful owner or authorized guest page/API access and accepted
captures extend expiry three days; an idle SSE connection does not. Cleanup
deletes expired bins and closes their streams. Each bin
accepts at most 500 captures and 100 MB (100,000,000 bytes) of raw request
bodies. Headers and metadata do not count. Common credential headers are
redacted before storage. Caddy's 10 MB (10,000,000-byte) body limit
applies to public capture routes; the
sharing update does not capture a request body. Public header and rate
limits are separate ingress controls.

Guest inspection endpoints are read-only. The public `/b/{code}` capture route
is separate: anyone who knows a bin code, including an invited guest, can send
requests to it. The bin replacement endpoint has been removed.
