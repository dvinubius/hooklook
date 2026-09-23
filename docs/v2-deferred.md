# Deferred V2 hardening

Historical note: the updated [production v1 plan](../.agents/PROJECT_PLAN.md)
now requires Caddy rate limiting and header-size enforcement. The policies
below describe the earlier plan and are not the current milestone direction.

## Edge rate limiting

Rate limiting is deferred from v1. Hooklook is expected to serve one developer
at a time, with no more than five actively used bins. The v1 body, header,
per-bin storage, request-count, and expiration limits provide the important
protection against accidental storage exhaustion at that scale.

Reconsider edge rate limiting if production observations show retry loops,
sustained high CPU or bandwidth, repeated per-bin storage rejections, or more
concurrent users than expected.

A v2 starting policy is per source IP, applied before proxying to hooklook:

| Route | Sustained rate | Burst | Response |
| --- | ---: | ---: | --- |
| `ANY /b/*` | 60 requests/minute | 20 requests | `429 Too Many Requests` with `Retry-After` |
| `POST /api/bins` | 10 requests/minute | 3 requests | `429 Too Many Requests` with `Retry-After` |

Caddy's standard directives do not provide a general request-rate limiter. A
v2 implementation therefore requires either a maintained, version-pinned Caddy
module or an upstream edge/firewall that can apply the policy. If Caddy is
behind another proxy or CDN, configure trusted proxies before using a forwarded
client-IP address as the key.

## Caddy request-header limit

This historical proposal is superseded in production: Caddy now enforces a
configured 32 KiB listener setting. Go's HTTP/1.1 parser adds a 4 KiB buffer
allowance, so the observed rejection boundary is approximately 36 KiB. The
setting remains shared because the Caddy instance serves other applications.

If a future dedicated Hooklook Caddy server is introduced, first verify the
limit is safe for its traffic. It would use:

```caddyfile
{
	servers {
		max_header_size 32KiB
	}
}
```

`max_header_size` is server-scoped. If unrelated sites require larger headers,
isolate hooklook on a dedicated listener or Caddy instance instead.

## Request-detail caching

Consider an in-memory frontend cache of individual request details, keyed by
bin code and request ID, so revisiting a request does not fetch its body again.
Prefer this over a server cache: SQLite remains the source of truth, and the
cache only serves the current browser session. A bin's raw bodies total at most
100 MB (100,000,000 bytes), but decoded data can use more browser memory, so
keep the cache bounded. Evict entries after deletion or clearing, discard the
bin's cache when access is revoked or the page changes bins, and reconcile
cached IDs with the refreshed request list after SSE reconnects. Do not persist
captured bodies in local storage or other browser storage.
