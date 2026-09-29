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
