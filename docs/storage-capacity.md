# Storage capacity and backups

## What is limited

A bin accepts at most 500 captured requests and 100 MB (100,000,000 bytes) of
raw request bodies. Headers and request metadata do not count toward that
per-bin byte budget. Deleting one request or clearing a bin reclaims its exact
body-byte allowance; the counter changes in the same SQLite transaction as the
deletion.

`MAX_STORE` is an optional positive decimal byte count, defaulting to
`5000000000` (5 GB). It limits the **main SQLite database file**, which stores
both metadata and captured payloads. At startup the service reads SQLite
`page_size` and requests `max_page_count` of `floor(MAX_STORE/page_size)`.
An existing database larger than that limit still opens: its bins remain
readable, cleanup and deletion can run, and new bins and captures receive
capacity errors. SQLite cannot set `max_page_count` below the file's existing
page count, so the application's write precheck enforces the configured limit
in that state. Once the file is below the limit, SQLite's page cap is the final
guard. Deletion makes pages reusable but does not necessarily shrink the file;
an already oversized file remains full until it is compacted or the limit is
raised.

## Capacity responses

Bin creation at global capacity serves the app document with `507 Insufficient
Storage`, `X-Hooklook-Error: store_full`, and
`data-hooklook-startup="store_full"` on `<html>`, without setting an owner
cookie. The marker lets the frontend recognize this state before it has a bin
code or can call the authorized metadata API. If frontend assets were not built,
a standalone explanation is served instead. A capture rejected by either
limit returns `507` without publishing an SSE event. Capture responses identify
the cause in both the body and `X-Hooklook-Error` header:

| Cause | Header value | Response text |
| --- | --- | --- |
| Per-bin request count or body-byte limit | `bin_full` | `bin storage limit reached` |
| Global SQLite capacity | `store_full` | `global storage limit reached` |

The per-bin limit is checked first when both limits are reached. Missing or
expired bins return `404` instead. A bin initially expires three days after
creation; owner use, authorized guest use, and accepted captures extend expiry
by three days. An idle SSE connection does not. Cleanup runs at startup and
once a minute, deleting expired bins and their requests and closing their SSE
streams. An expired bin page explains the expiration and waits for the visitor
to choose **Create New Bin**. See [bin lifecycle](bin-lifecycle.md) for the full
retention flow.

Authorized `GET /api/bins/{code}` responses include current per-bin counts,
limits, and `requestsFull`, `bodyBytesFull`, and `full` flags in a `capacity`
object. A sibling `storeCapacity` object reports the global SQLite page budget,
including its `full` flag. The global flag uses the minimum two-page allowance
needed by the write precheck; a larger capture may fail before it turns true.
See the [HTTP API](http-api.md) for exact fields and how to refresh the state
after an SSE event.

Operators can read the global totals without access to a bin through
bearer-protected `GET /admin/storage`. In addition to the capacity fields above,
it reports live-page `usedBytes` and `usedPercent` of `maxBytes`. See the
[HTTP API](http-api.md) for the exact response semantics.

The frontend shows all three states. A marked `507` document renders an apology
between the page's own bars and calls no API at all. A bin's own fullness is a
gauge on its request list, which reads 100% only when the `full` flag is set. A
bin page whose response reports `storeCapacity.full` says so in brick at the top
of the page, whatever room that particular bin still has. See
[frontend](frontend.md) for the wording and the refresh behavior.

## Filesystem headroom

The configured `MAX_STORE` ceiling covers only the main SQLite database file.
For production, the database directory is stored in the persistent
`hooklook-data` Docker named volume mounted at `/data`. SQLite journals,
temporary files, backup staging, container images, and the other services on
the VM can consume space outside the database page cap. Monitor both database
page/file use against `MAX_STORE` and free space on the host filesystem when
the observability milestone is implemented. Backups must be written outside
the live data volume and copied off the host.

Before starting or recreating the production service, run
`scripts/check-storage-headroom.sh`. It checks the filesystem that contains
Docker's data root and requires at least `2 × MAX_STORE + 2,000,000,000` bytes
free: one full configured database-growth allowance, one SQLite backup staging
copy, and 2 GB for images, journals, and ordinary deployment work. With the
default `MAX_STORE`, that is 12 GB. The later deployment script calls this same
check before `docker compose up`.

The application is published only on VM loopback; public traffic reaches it
through the separately managed Caddy ingress. Its body-size, header-size, and
rate limits are separate from the SQLite page cap.

## Online backup

The `backup` command uses SQLite `VACUUM INTO` to produce a consistent backup
while the service is running. Use the deployment wrapper, which selects a
timestamped destination outside the persistent volume, creates a checksum, and
protects both files with mode `0600`. The backup is a standalone SQLite
database with no separate payload directory; protect it like the live volume.
See the [database backup runbook](database-backup-runbook.md) for transfer,
storage, verification, and restore-drill guidance.

## Restore

Periodically test a restore in a separate location. The runbook uses the
application's `integrity-check` command, an alternate loopback port, and
representative authorized reads without touching Caddy or production data. For
a production restore, stop the service, retain the current database as a
rollback copy, copy the backup to `hooklook.db` on the persistent volume, set
ownership and permissions for the service user, then start the service. Do not
replace a live database file because the service holds an open SQLite
connection. If the restored file exceeds `MAX_STORE`, the service starts with
creation and capture blocked until the file is compacted or the limit is
raised.

See the [HTTP API](http-api.md) for route responses and the
[architecture](architecture.md) for the storage components.
