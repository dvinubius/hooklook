# hooklook

A small, self-hosted request bin written in Go. Hooklook token holders can
create temporary endpoints, send arbitrary HTTP requests, and inspect what was
captured live.

This is a learning project focused on Go's HTTP model, safe handling of
untrusted payloads, SQLite persistence, live Server-Sent Events, and a small
complete service.

## Project guidance

- [Plan](.agents/PROJECT_PLAN.md)
- [Current progress](.agents/PROGRESS.md)

## Status

Milestone 1 is complete: bins and captured request summaries are held in memory
and available through the HTTP API. The public product requirements and
delivery plan are documented.

## Current next step

Capture request data faithfully: raw query, headers, body, content type, and
useful request metadata.

## Header redaction policy

When header capture is added, hooklook will redact every value for these
case-insensitive header names before persistence: `Authorization`,
`Proxy-Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key`,
`X-Auth-Token`, and `X-Access-Token`. The header name and number of values are
retained, but each value is stored as `[REDACTED]`.

Webhook signature headers are retained in v1 because they are important when
debugging signature verification. Treat a bin as sensitive while it exists:
signature values, request bodies, query parameters, and other unredacted
headers may be visible to anyone able to access the bin.

## Local configuration

`PUBLIC_BASE_URL` is required to create bins because it is used to construct
their inbound URLs. For local development:

```bash
set -a
source .env
set +a
go run .
```

Hooklook reads process environment variables only; `.env` is a local shell and
deployment convenience, not an application configuration format.
