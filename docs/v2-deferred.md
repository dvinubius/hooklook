# Deferred V2 hardening

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

## Off-host R2 backup automation

Create a private Cloudflare R2 bucket for Hooklook backup pairs (db + sha256).
Do not make it public or serve application traffic from it.

Add a production-host scheduled job with an overlap lock. It must call the
SQLite-aware backup wrapper, run `integrity-check` against the newly created
snapshot on Hetzner-One, then upload the `.db` and `.sha256` files under one
timestamped R2 object prefix.

Treat successful uploads of both objects as the source-side transfer result.
On any failure, retain local staging for investigation, write non-sensitive
diagnostics, and signal the failure to the chosen operations channel.
