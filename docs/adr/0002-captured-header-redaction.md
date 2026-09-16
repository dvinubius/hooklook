# ADR 0002: Redact credential-bearing request headers before persistence

## Status

Accepted

## Context

Hooklook retains incoming request headers so a developer can diagnose webhook
delivery. Headers often contain credentials or session material, and bins are
publicly reachable capability-like links rather than a robust authorization
boundary. Persisting such values would turn a useful debugging tool into an
unnecessary credential store.

At the same time, some webhook signature headers are necessary when debugging
signature verification. Removing all headers would reduce hooklook's value for
its core use case.

## Decision

Before persistence, redact every value for these case-insensitive header names:

- `Authorization`
- `Proxy-Authorization`
- `Cookie`
- `Set-Cookie`
- `X-Api-Key`
- `X-Auth-Token`
- `X-Access-Token`

Keep each original header name and the number of its values, but replace every
stored value with `[REDACTED]`. Apply the rule to every repeated value, not only
the first.

Retain webhook signature headers in v1. They are useful for reproducing and
debugging signature verification. This means every bin remains sensitive:
request bodies, query parameters, unredacted headers, and signatures can be
visible to anyone who can access that bin.

## Consequences

- Hooklook avoids persistently storing common credential-bearing headers while
  retaining enough request shape to debug integrations.
- Redaction happens before the storage implementation, so the same behavior
  applies to both the current in-memory store and SQLite.
- The policy is intentionally an allowlist of known sensitive header names, not
  a claim that all secrets are automatically detected. Users must not send
  sensitive production data to a bin unless they accept the retention risk.
