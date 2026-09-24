# hooklook

A small, self-hosted request bin written in Go, helping you to test and debug
webhook integrations. The backend creates one cookie-associated bin per browser,
captures arbitrary HTTP requests, and provides owner or invited-guest inspection
APIs. The Go binary serves the built Vue inspector frontend on its authorized
bin pages, so one binary is the whole deployment.

## Intended v1 use

Hooklook is designed for small-scale integration testing of systems using
webhooks. A bin lasts three days after its last use and holds up to 500 requests
and 100 MB of request bodies. See [bin lifecycle](docs/bin-lifecycle.md) and
[storage capacity](docs/storage-capacity.md) for the limits and retention rules.

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
- **A bin says how full it is.** The capture row carries a capacity gauge with
  a reading per limit — stored bytes and request slots — each green until 90%
  and brick past it. When the service as a whole is out of storage, the page
  says so in brick, and a visitor who cannot be given a bin at all gets a page
  that apologizes instead of a broken one. See
  [storage capacity](docs/storage-capacity.md) for the limits themselves. The
  gauge is the owner's; see below for why a guest is shown none of it.
- **Owners get controls; guests get none.** Guest access on or off and the
  invitation link sit under the share button; clearing all requests is the
  sweep button; deleting one request is on the request itself. The help button,
  which explains capturing, sharing and how long a bin is kept, is the owner's
  too. Clearing keeps the bin, its capture URL and its invitation. Hiding the
  controls is presentation only — the server re-checks ownership, the cookie
  and the request origin on every mutation.

A shared link is read-only, and it is the invitation — not the bin code — that
grants it. Disabling sharing stops that link working and closes any guest stream it
has open; the owner's own page stays live. Enabling it again makes the same
link work.

**Guest mode is for showing someone the requests, not the plumbing.** The
person you send a link to is usually not the developer working on the webhook
delivery — a product owner checking that the payload carries the field they
asked for, a partner's integrator confirming what they sent, someone in support
attaching evidence to a ticket. They came to read one request, so their page is
the requests and nothing else: no capacity gauge, no help dialog, no controls.
Bin stats are housekeeping for whoever owns the bin, and how full it is has no
bearing on what a guest came to read — the owner is the one who can act on it,
and a number a reader cannot act on is a number in the way. What a guest needs
from the page is on the page already: the requests, and the fact that nothing
can be changed.

## Project documentation

- [Documentation routing index](docs/README.md)
- [Plan](.agents/PROJECT_PLAN.md)
- [Current progress](.agents/PROGRESS.md)
- [Completed milestones](.agents/done-milestones.md)
- [Current architecture](docs/architecture.md)
- [Observability](docs/observability.md)
- [Deployment security](docs/security.md)
- [Database behavior](docs/db.md)
- [Bin access](docs/bin-access.md)
- [Bin lifecycle](docs/bin-lifecycle.md)
- [Storage capacity and backups](docs/storage-capacity.md)
- [Production verification runbook](docs/production-verification-runbook.md)
- [Database backup runbook](docs/database-backup-runbook.md)
- [Current HTTP API](docs/http-api.md)
- [Frontend behavior and mechanisms](docs/frontend/frontend.md)
- [Architecture decision records](docs/adr/)

## Local development

`PUBLIC_BASE_URL` is required to construct capture URLs. `LISTEN_ADDRESS`
defaults to `127.0.0.1:8080` for local development; set it to an explicit
host-and-port address when a deployment needs another listener. `ADMIN_TOKEN`
is required to start the server and protects
the `GET /admin/bins` and `GET /admin/storage` operator routes through a bearer
token. Configuration comes from process environment variables; `.env` is a
shell convenience, not an application configuration format.

`MAX_STORE` optionally changes the SQLite database limit from its 5 GB default.
See [storage capacity](docs/storage-capacity.md) for its byte definition and
capacity behavior.

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

### Container package

`Dockerfile` builds the Vue application before compiling the CGO SQLite binary,
then runs the resulting binary as the fixed non-root `hooklook` user. The
`compose.yaml` service keeps its root filesystem read-only, gives it a bounded
`/tmp` tmpfs, and persists `/data` in the `hooklook-data` named volume.

Compose requires `PUBLIC_BASE_URL` and `ADMIN_TOKEN`, uses the 5 GB default for
`MAX_STORE`, and sets `LISTEN_ADDRESS=0.0.0.0:8080` only inside the container.
It publishes the application solely as `127.0.0.1:8081` and joins the
Caddy-owned external `hooklook-edge` network. Do not create or manage that
shared network from this project.

### Deploying to the VPS

Deploy a prepared VPS with [`scripts/deploy.sh`](scripts/deploy.sh). It tests
locally before contacting the host and deploys the Hooklook service by default.
Use `DEPLOY_PROFILE=full` with separate Hooklook Grafana credentials to include
the private telemetry stack. After the first full deployment,
`DEPLOY_PROFILE=observability` updates only telemetry configuration and services,
and `DEPLOY_PROFILE=dashboard` uploads only the operator dashboard JSON. Full
and observability deployments run telemetry smoke checks; dashboard updates
check Grafana provisioning. See the [deployment runbook](docs/deployment-runbook.md) for prerequisites,
configuration, rollout, verification, diagnostics, and rollback.

## Tests

```bash
make test        # frontend (vitest) and Go tests
make test-race   # Go race detector
make test-deploy # deployment safety-gate behavior
make test-backup # backup-wrapper safety behavior
make test-verify-public # public-verifier input-safety behavior
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

## Observability

The app exposes `/health` for liveness and `/ready` for a bounded SQLite check.
A separate listener (`METRICS_LISTEN_ADDRESS`, default `127.0.0.1:9092`)
serves `/metrics`; Docker configures it as `0.0.0.0:9092` inside Hooklook's
container so Prometheus can scrape it. It has no host port or Caddy route.
Because Hooklook also joins `hooklook-edge`, Caddy can reach this listener
directly over Docker even though the public cannot. HTTP labels use fixed
route and method classes; captured paths, query strings, headers, and bodies
stay out of metrics and logs. The private stack is Prometheus, Alloy, Loki, and
Grafana, enabled with the Compose `observability` profile. Grafana binds only
`127.0.0.1:3001` on the host. See the
[observability runbook](docs/observability-runbook.md) for local validation,
operator access, retention, smoke checks, and rollback.
