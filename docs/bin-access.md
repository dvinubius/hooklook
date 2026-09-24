# Bin access

## Browser ownership

A visit to `/` resolves the browser's existing bin or creates one, then
redirects to `/bins/{code}`. Bin codes have readable adjective-noun-number
addresses. Each bin has a cryptographically random ownership secret. SQLite
stores its SHA-256 digest; the browser receives the secret in an `HttpOnly`,
`SameSite=Lax` cookie, marked `Secure` when `PUBLIC_BASE_URL` uses HTTPS. The
cookie's expiry is refreshed when the owner uses the bin.

A visitor to another active bin needs a valid invitation. When a target is
unavailable, the link shape determines the framed empty-state message. A URL
with a nonempty `?invite=...` says that the shared bin no longer exists,
whether the invitation is invalid or revoked or the bin is missing or expired.
A regular bin URL says that the bin expired or no longer exists. Both states
wait for the visitor to choose **Create New Bin**, which leads through `/`. A
bin code alone does not grant inspection access.

## Guest invitations

Each bin has one reusable invitation identifier. The owner can enable or
disable sharing without changing that identifier. A guest supplies it as
`?invite=...` on the page and each inspection API or SSE request. While sharing
is enabled, guests can read the bin but cannot delete requests, clear the bin,
or change sharing. Disabling sharing revokes guest API access and closes guest
SSE streams while leaving the owner's stream open.

The metadata, list, detail, and SSE routes check owner or guest access. Owner
mutations also require an `Origin` matching `PUBLIC_BASE_URL`. Page and access
responses use `Cache-Control: no-store` and `Referrer-Policy: no-referrer`.
The frontend fetches `GET /api/bins/{code}` before showing private content; it
does not infer access merely from having rendered a page.

## Public capture versus inspection

Anyone who knows a bin code can send requests to `/b/{code}`, including a
guest. The invitation controls inspection, not capture. Captured bodies are
stored as bytes and returned in JSON as base64; the frontend renders those
bytes as text, never as markup. The capture-reported detail URL is authorized
like the bin page. There is no bin replacement endpoint.

See [bin lifecycle](bin-lifecycle.md) for retention,
[storage capacity](storage-capacity.md) for limits, the [HTTP API](http-api.md)
for route details, and [frontend](frontend/frontend.md) for the client's access and
rendering behavior.
