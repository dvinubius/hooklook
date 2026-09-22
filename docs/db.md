# Database behavior

## Role and connection setup

SQLite is hooklook's source of truth for bins, captured requests, ownership,
sharing, expiry, and stored-byte counters. The application opens
`hooklook.db` through `database/sql` and `github.com/mattn/go-sqlite3`.

The connection setup:

- enables foreign keys, so deleting a bin cascades to its requests;
- waits up to five seconds for a busy database before returning an error;
- limits the pool to one open connection, matching the service's deliberately
  small, single-process workload; and
- pings SQLite before startup continues.

Startup also creates the development schema and configures the SQLite page cap
from `MAX_STORE`. Any open, ping, schema, or capacity-configuration failure
prevents the HTTP server from starting.

## Development schema

The `bins` table stores the readable bin code, creation and expiry times, the
current raw-body byte total, the hashed owner secret, the reusable invitation
identifier, and whether sharing is enabled.

The `requests` table stores each request's bin, receipt time, HTTP fields,
redacted headers, content type, raw body, and displayed body size. Its foreign
key uses `ON DELETE CASCADE` so bin cleanup removes the associated requests.

Request IDs use `INTEGER PRIMARY KEY AUTOINCREMENT`. SQLite therefore assigns
an ID greater than every committed ID previously generated for that table,
including IDs of rows that were later deleted. IDs can still contain gaps, for
example after a failed insertion. IDs allocated by rolled-back transactions may
be reused. They are identifiers rather than a count of current requests.

The current schema setup uses `CREATE TABLE IF NOT EXISTS`; it is intended for
fresh development databases and does not upgrade an existing table definition.
There is no migration framework yet because no production database exists.

## Transactions and consistency

Saving a capture updates its bin's stored-byte counter and inserts the request
in one transaction. Deleting one request removes it and subtracts its exact raw
body length in one transaction. Clearing a bin deletes all of its requests and
resets the counter in one transaction. A committed database change happens
before an SSE notification is published, so reconnecting clients can always
rebuild their view from SQLite.

Authorization that renews a bin is also one conditional SQLite statement. This
prevents cleanup from deleting the bin between checking access and extending
its expiry. See [bin access](bin-access.md) and
[bin lifecycle](bin-lifecycle.md) for the user-visible behavior.

## Runtime database failure

SQLite is embedded, so hooklook has no remote database server or network
connection to lose. `database/sql` may replace an internal driver connection;
that is not a service failure when the database remains usable.

The running service checks the database once a second with
`PRAGMA schema_version` through the same pool used by requests. If that check
fails, hooklook:

1. stops the database monitor and expiration worker;
2. closes active SSE streams;
3. gives the HTTP server up to ten seconds to shut down gracefully; and
4. returns an error from the server run loop, causing the process to exit with
   a nonzero status.

An expiration-cleanup database error enters the same shutdown path immediately.
A request that encounters the failure between checks may receive `500 Internal
Server Error` before the next check stops the process.

Expected application outcomes do not stop the process. These include a missing
or expired bin, a missing request, a full bin, and a full global store. Storage
capacity has deliberate `507 Insufficient Storage` responses; see
[storage capacity and backups](storage-capacity.md).

## Operations

Deleting rows can make SQLite pages reusable without shrinking the main file.
`MAX_STORE`, capacity reporting, filesystem headroom, backup, and restore are
documented in [storage capacity and backups](storage-capacity.md). Do not
replace the live database file while hooklook is running because the process
holds it open.
