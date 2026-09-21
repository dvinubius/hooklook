# hooklook

A small, self-hosted request bin written in Go, helping you to test and debug
webhook integrations. The backend creates one cookie-associated bin per browser,
captures arbitrary HTTP requests, and provides owner or invited-guest inspection
APIs. The Go binary serves the built Vue inspector frontend on its authorized
bin pages, so one binary is the whole deployment.

## Intended v1 use

Hooklook is designed for small-scale integration testing of systems using
webhooks. The current limits and planned production limits differ; see the
[current architecture](docs/architecture.md) and
[production design notes](.agents/design-notes.md).

## Live request events

`GET /api/bins/{binCode}/events` opens an authorized Server-Sent Events stream.
Owners send their cookie; guests append their invitation identifier as
`?invite=...`. Each successfully persisted inbound request produces one
`request` event whose `data` is the same compact request summary returned by the
request-list endpoint. Deletion and clearing publish a `refresh` event. Events
are intentionally ephemeral: clients must refetch the list after reconnecting.

Each subscriber has a one-event buffer. If it cannot consume the next event,
hooklook closes that stream immediately; this prevents a slow browser from
blocking ingestion or accumulating unbounded work. Deleting a bin and graceful
server shutdown also close its open event streams.

## Inspecting a webhook bin

Open `/` and hooklook resolves or creates the bin your browser owns, then takes
you to its page. The page shows the capture URL to send requests to, the live
list of what has arrived, and the full detail of whichever request is selected.
A `curl` example to try the URL with is in the help dialog.

- **The list updates itself.** Captures appear without a reload, over the SSE
  stream. Because the stream replays nothing, the list is refetched from SQLite
  after every reconnect and whenever a disconnected tab comes back to the
  foreground, so a tab that was asleep converges instead of drifting.
- **Sort and filter are local** — newest or oldest first, a method filter,
  and path and raw query filters that match a substring or test for empty or
  not empty. Live captures keep arriving while a filter is on.
- **The selected request is in the URL**, so back, forward and the detail link
  a capture returns (`/bins/{code}/requests/{id}`) all select the same one.
- **Bodies are shown as the bytes they are.** JSON and XML are pretty-printed
  and highlighted; a body that is not valid says so and keeps its raw view.
  Text that is not UTF-8 is decoded with a named fallback, and the encoding
  used is stated. Bytes that are not text get a hex dump. Nothing captured is
  ever rendered as markup.
- **A capture that came through the proxy shows its client address.** When
  the request carried an `X-Forwarded-For`, the detail reports a **client ip**
  beside the received time. It is read off that stored header — no address is
  captured or stored on its own — and it is the last hop, the one our own
  ingress accepted, not the first one a caller can put there themselves.
- **Redacted headers stay redacted.** The values were replaced before storage,
  and a note by the headers says so rather than implying they could be
  recovered. Bodies, by contrast, are stored as received, which the body's own
  note states.
- **Owners get controls; guests get none.** Guest access on or off and the
  invitation link sit under the share button; clearing all requests is the
  sweep button; deleting one request is on the request itself. The help button
  explains capturing, sharing and how long a bin is kept. Clearing keeps the
  bin, its capture URL and its invitation. Hiding the controls is presentation
  only — the server re-checks ownership, the cookie and the request origin on
  every mutation.

A shared link is read-only, and it is the invitation — not the bin code — that
grants it. Disabling sharing stops that link working and closes any guest stream it
has open; the owner's own page stays live. Enabling it again makes the same
link work.

## Project documentation

- [Plan](.agents/PROJECT_PLAN.md)
- [Current progress](.agents/PROGRESS.md)
- [Completed milestones](.agents/done-milestones.md)
- [Current architecture](docs/architecture.md)
- [Current HTTP API](docs/http-api.md)
- [Frontend behavior and mechanisms](docs/frontend.md)
- [Architecture decision records](docs/adr/)

## Local development

`PUBLIC_BASE_URL` is required to construct capture URLs. The server binds to
`127.0.0.1:8080`. `ADMIN_TOKEN` is required to start the server and protects
`GET /admin/bins` through a bearer token. Configuration comes from process
environment variables; `.env` is a shell
convenience, not an application configuration format.

### Working on the frontend

Development runs two servers. The browser stays on the Vite origin, which
proxies the home, bin page, API, SSE and capture routes to Go, so the owner
cookie, the same-origin check on mutations and `EventSource` all see one
origin — and Go still authorizes every page before Vite's modules load.

```bash
make dev-go    # Go, serving the Vite-backed page shell
make dev-web   # Vite; open http://localhost:5173, not the Go port
```

### Running a production build

The binary embeds `frontend/dist`, so the frontend has to be built *before* Go
compiles. `make build` does both in the required order; building Go on its own
after an unbuilt checkout produces a binary whose bin pages report the missing
build instead of serving a blank document.

```bash
make build         # npm run build, then go build -o webhook-inspector
set -a
source .env
set +a
export ADMIN_TOKEN=your-local-operator-secret
./webhook-inspector
# Open http://localhost:8080/ in a browser to create a bin
# Optional: ./webhook-inspector dev-bin prints a fixture capture URL
```

The `dev-bin` command creates a fixture directly in SQLite. Normal browser use
starts at `/`, which creates or reuses a bin and redirects to its inspector
page. See the [HTTP API](docs/http-api.md) for inspection and owner mutation
routes.

The backend enforces 500 requests and 100 MB (100,000,000 bytes) of raw request
bodies in total per bin. Headers and metadata do not count. The Go-specific body
and header policy limits have been removed. Keep this build on loopback;
configure Caddy body, 32 KiB total-header, and rate limits before exposing it
publicly.

## Tests

```bash
make test        # frontend (vitest) and Go tests
make test-race   # Go race detector
make vet
```

Frontend tests run in Node. The component tests render through Vue's own server
renderer rather than a browser, which is what checks that a captured body
reaches the page as characters and never as markup. Browser-visible behavior is
reviewed in a browser by hand.

## Inspection exercise

Exercises the whole flow against the built binary — first visit, capture of
every body shape, list, detail, the live stream, owner mutations, guest
invitation and revocation — and checks that no owner cookie or
invitation reaches the log:

```bash
make build
./scripts/exercise-inspection.sh
```

It starts its own server over a throwaway database in a temporary directory,
so it leaves the repository's `hooklook.db` alone, and it cleans both up on the
way out. The server binds `127.0.0.1:8080`. Every check runs even after one
fails; the exit status is the number of failures. `BINARY` overrides the
binary it runs.

## Capture smoke test

With the server running locally and a bin already created, exercise the current
Milestone 2 capture shapes with:

```bash
./scripts/exercise-capture.sh YOUR_BIN_CODE
```

Set `BASE_URL` to test a different instance, for example
`BASE_URL=http://localhost:8081 ./scripts/exercise-capture.sh YOUR_BIN_CODE`.
The script sends capture requests only; inspect the bin separately.

## Header redaction policy

Hooklook redacts common credential-bearing headers before storing a request.
Webhook signature headers are retained for debugging, so treat every bin as
sensitive. See [ADR 0002](docs/adr/0002-captured-header-redaction.md) for the
exact header list, repeated-header behavior, and security implications.
