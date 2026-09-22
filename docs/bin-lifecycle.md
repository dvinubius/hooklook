# Bin lifecycle

## Creation and capture

A new bin expires three days after creation. Public `/b/{code}` accepts
arbitrary HTTP methods and optional path suffixes while the bin is active.
Capture preserves the method, path suffix, raw query, redacted headers, body,
content type, and receipt time. The request is persisted before its summary is
published over SSE. Missing or expired bins return `404` and publish nothing.

Accepted captures count toward the bin's request and body-byte limits. Deleting
one request or clearing all requests reclaims its allowance and publishes an
SSE refresh signal. See [storage capacity](storage-capacity.md) for the exact
budgets and rejection responses.

## Renewal and expiration

Owner use, authorized guest page/API use, and accepted captures move expiry to
three days after that use. Authorization and renewal are a conditional SQLite
write, and accepted captures update expiry in their insertion transaction. An
open SSE stream alone does not continually renew the bin.

The application-owned cleanup worker runs at startup and once a minute. It
deletes expired bins and their requests through SQLite's foreign-key cascade,
then closes associated SSE streams. A regular page URL for an expired or
already deleted bin shows an explicit expiration page before the visitor
chooses **Create New Bin**. A shared URL with a nonempty `invite` parameter
instead says that the shared bin no longer exists. An expired capture returns
`404`.

See [bin access](bin-access.md) for ownership and invitations and the
[HTTP API](http-api.md) for exact routes and responses.
