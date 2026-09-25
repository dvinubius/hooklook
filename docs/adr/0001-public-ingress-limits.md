# ADR 0001: Bound public ingress by size, lifetime, and storage

## Status

Superseded for the production v1 target by the project's design notes. This
ADR records the
limits implemented in the current milestone-5 code.

## Context

Hooklook is a free, public, self-hosted portfolio application for inspecting
webhooks. It is designed for a developer to test an integration, not to accept
file uploads, retain an event archive, or operate at internet scale.

The inbound endpoint of an issued bin is public. Token-protected bin creation
limits who can create bins, but it does not limit the sources that can send to
an existing bin. Size, duration, and storage boundaries are
therefore product requirements for predictable operation on a small VPS.

## Decision

### Resource policy

| Resource | Limit | Expected-use rationale |
| --- | ---: | --- |
| Captured request body | 256 KiB | Normal JSON, XML, form, text, and small binary webhook payloads fit; file-like payloads do not. |
| Captured requests per bin | 500 | More than enough for an integration test while keeping the bin inspectable. |
| Captured body bytes per bin | 10 MiB | Allows forty maximum-size captures or many ordinary webhook events without giving one bin unbounded disk use. |
| Bin ingestion lifetime | 3 days | Generous for integration testing while preventing forgotten public endpoints from accepting traffic indefinitely. |
| Production Caddy request-header setting | 32 KiB configured; ~36 KiB effective for HTTP/1.1 | Go adds a 4 KiB parser-buffer allowance, while still bounding header parsing and persistence exposure. |

Bodies over 256 KiB receive `413 Payload Too Large`; HTTP/1.1 headers above
the approximately 36 KiB effective Caddy boundary receive `431 Request Header
Fields Too Large`. Both are rejected before
persistence. Once SQLite is introduced, request-count and body-byte checks are
performed in the same transaction as the request insert. A bin at either budget
receives `507 Insufficient Storage` with a clear explanation and no partial
capture.

At expiration, a bin stops accepting inbound requests and cleanup deletes the
bin and its captured data. V1 has no separate post-expiration viewing period.

### Server timeouts

The Go server applies a 5-second `ReadHeaderTimeout`, 15-second `ReadTimeout`,
and 60-second `IdleTimeout`. These bound slow or stalled connections while
leaving ample time for a 256 KiB webhook request to traverse the local
Caddy-to-Go hop. `WriteTimeout` remains unset because a server-wide write
deadline would interrupt the planned long-lived SSE endpoint; Caddy owns the
public client connection and its response behavior.

## Consequences

- Hooklook clearly rejects payloads and traffic beyond its advertised capacity
  rather than failing unpredictably.
- The inspection UI lists summaries and fetches a selected request body only.
- SQLite needs explicit `request_count`, `captured_body_bytes`, and per-request
  `body_size` data so the bin budgets are auditable and transactionally safe.
- Edge rate limiting is deferred to v2; see
  [`docs/v2-deferred.md`](../v2-deferred.md).

## References
