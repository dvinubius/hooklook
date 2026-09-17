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

The Go server's 32 KiB header limit is sufficient for v1's expected use and
protects the application before it captures or persists a request. Matching
Caddy enforcement is deferred because the Caddy instance is shared with other
applications and its server-level setting may affect them.

If v2 operation warrants it, first verify the limit is safe for all sites on
the affected Caddy HTTP server. For a dedicated hooklook Caddy server, use:

```caddyfile
{
	servers {
		max_header_size 32KiB
	}
}
```

`max_header_size` is server-scoped. If unrelated sites require larger headers,
isolate hooklook on a dedicated listener or Caddy instance instead.
