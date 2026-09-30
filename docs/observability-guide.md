# Using the Hooklook dashboard

This guide teaches you to answer real questions with **Hooklook → Hooklook
operator**. The [observability overview](observability.md) documents the stack
and every metric; the [observability runbook](observability-runbook.md) covers
validating, deploying and restoring it. This document covers reading it.

Open the dashboard through the SSH tunnel described in the
[observability runbook](observability-runbook.md#deployment-preparation-and-private-access).
It opens on the last 24 hours and refreshes every 30 seconds.

The operator dashboard counts all traffic, including the private
synthetic-traffic generator. Two more dashboards in the Hooklook folder split
that off, and a fourth is the metrics-only view shared with the public:

| Dashboard | Filter | Use it to |
| --- | --- | --- |
| **Hooklook operator** | None | Triage anything; the only view with storage, SQLite errors and pool, cleanup, runtime, and logs across all traffic |
| **Hooklook synthetic traffic** | `traffic_class="synthetic"` | Check that the generator runs and behaves |
| **Hooklook non-synthetic traffic** | `traffic_class="other"` | Approximate real use: people, bots, scanners, health checks |
| **Hooklook public metrics** | None, aggregated | See what the public sees; never use it to diagnose |

The class dashboards repeat the operator's request, latency, capture, and
bin-operation panels with the same names, and add **Bins created** and
**Capacity rejections** totals. Their logs panel shows `http_request` lines for
that class. See [traffic classes](observability.md#traffic-classes) for the
rule and its limits. The public dashboard's content and boundary are described
in [public dashboard](observability.md#public-dashboard).

## 1. What the dashboard can see

The dashboard sees Hooklook from inside the Go process, one hop behind Caddy.
It counts each request the application finished, labelled with a **route
class**, the method and the status code. It knows **which kind of request** and
**what outcome**, never **which bin**, **which path** or **which client**. Bin
codes, raw paths, IP addresses and payloads are deliberately kept out of both
metrics and logs.

Some traffic never reaches it:

- Everything Caddy answers itself: the rate-limit `429`, the `413` for a
  declared body over 10 MB, the `431` for oversized headers, TLS handshakes
  that never become a request, and the `502` Caddy returns while Hooklook is
  down. Those live on the Caddy dashboard (see [section 9](#9-when-to-leave-this-dashboard)).
- Prometheus's own scrapes of `:9092`. They use a separate listener and are
  not HTTP traffic.
- The backup and integrity-check commands, which run as separate processes.

### Route classes

Every request is filed under one of these classes, taken from the URL before
any handler runs:

| Route class | Requests |
| --- | --- |
| `home` | `GET /` and `GET /?cookie-check`: returns your bin, or checks cookies and creates one, then redirects |
| `bin_page` | `/bins/{code}…` page documents and their redirects |
| `api_bin` | `GET /api/bins/{code}`: bin metadata and capacity |
| `api_request_list` | `GET` (list) and `DELETE` (clear) on `/api/bins/{code}/requests` |
| `api_request_detail` | `GET` and `DELETE` on one captured request |
| `api_sharing` | `PUT /api/bins/{code}/sharing` |
| `sse` | `/api/bins/{code}/events`, the live stream |
| `capture` | `/b/{code}…` with any method: the webhooks |
| `static` | Built assets, fonts, favicon, logo |
| `health` | `/health` |
| `admin_bins`, `admin_storage` | The bearer-protected operator API |
| `other` | `/ready`, and every path no handler knows, such as scanners probing `/wp-login.php` |

### How each kind of event is recorded

Most scenarios below are applications of this table. Each row says where one
event shows up. A dash means it leaves no trace there.

| What happened | HTTP series (route, status) | Capture result | Bin operation | SQLite outcome | Log line |
| --- | --- | --- | --- | --- | --- |
| Webhook stored | `capture 201` | `accepted` | `capture` | `capture success` | `http_request` |
| Webhook to an unknown or expired bin | `capture 404` | `missing_bin` | — | `capture not_found` | `http_request` |
| Webhook to a bin at 500 requests or 100 MB | `capture 507` | `bin_full` | — | `capture capacity` | WARN `capture_capacity_rejected` |
| Webhook while SQLite is at `MAX_STORE` | `capture 507` | `store_full` | — | `capture capacity`; `capture error` kind `full` if SQLite itself refused | WARN `capture_capacity_rejected` |
| Chunked body cut off by Caddy at 10 MB | `capture 400` | — | — | — | `http_request` |
| Declared body over 10 MB, rate limit, huge headers | — | — | — | — | — |
| First visit to `/`, no owner cookie (browser, or any client that keeps cookies) | `home 303` ×2 | — | `create` | `create success` | `http_request` ×2 |
| Returning visit to `/` with a cookie | `home 303` | — | — | — | `http_request` |
| Visit to `/` from a client that drops cookies | `home 303`, then `home 200` | — | — | — | `http_request` ×2, `bin_creation_cookie_missing` |
| First visit while SQLite is at `MAX_STORE` | `home 303`, then `home 507` | — | — | `create capacity` | WARN `bin_creation_capacity_rejected` |
| Owner opens a bin page in a browser | `bin_page 200`, `api_bin 200`, `api_request_list 200` | — | `list` | `list success` | `http_request` ×3, `sse_opened` |
| Page for an expired or foreign bin | `bin_page 404` | — | — | — | `http_request` |
| Owner clicks one captured request | `api_request_detail 200` | — | `detail` | `detail success` | `http_request` |
| Live stream ends (tab closed, bin expired, restart) | `sse 200`, **only now** | — | — | — | `sse_closed` with a reason |
| Cleanup deletes expired bins | — | — | — | — | `cleanup_completed` (only if any were deleted) |

Four consequences are worth memorising:

1. **Capture results explain capture statuses.** A `capture 404` is always a
   `missing_bin`; a `capture 507` is `bin_full` or `store_full`; a
   `capture 500` is `internal_error`. A `capture 400` has no capture result at
   all, because the body never finished arriving.
2. **"SQLite errors" are only SQLite failures.** A missing bin or request
   is result `not_found`, and a full bin or a store rejected by the capacity
   pre-check is result `capacity`, in **SQLite operation outcomes**. Neither
   reaches **SQLite errors**. What does is a query that failed: kind `full`
   is SQLite itself refusing to grow (its page cap or a full disk, which the
   pre-check did not see coming), kind `other` is everything else. Any bar
   there is worth a look.
3. **A stream is counted when it ends, not when it starts.** An `sse`
   request is invisible to the traffic counter for as long as the tab stays
   open. It is also excluded from latency and in-flight panels, which would
   otherwise be dominated by hour-long streams.
4. **Only a client that keeps cookies gets a bin.** A cookieless visit to
   `/` first sets a short-lived cookie check and redirects to
   `/?cookie-check`; the bin is created there, and only if the check came
   back. A crawler that drops cookies leaves `home 303` then `home 200` (the
   "Hooklook needs cookies" page) and no bin. Link-preview bots (Slack,
   Discord, and so on) are recognised earlier and get a plain `home 200`.
   `hooklook_bin_creation_results_total` (see [section 7](#7-explore-recipes))
   splits the attempts into `created`, `no_cookie` and `store_full`.

### Reading rules

- **The top-row stats cover the whole selected range.** At "Last 7 days",
  **Requests** and **Application 5xx** are weekly totals. During an incident,
  set the range to 15 or 30 minutes so they describe now. The other stats
  (**Active bins**, **Open SSE streams**, the storage stats) are current
  values, whatever the range. **Expired bins deleted, 24 hours** is always the
  last 24 hours.
- **"24 hours" and "7 days" panels are rolling windows, not bars.** **Bin
  operations**, **SSE events**, **Capacity rejections by cause**, **SQLite
  operation outcomes**, **SQLite errors** and **Expiry cleanup runs** plot, at
  each moment, the total of the 24 hours (or 7 days) before it. A burst of 50
  errors at 10:00 draws a step up of 50 at 10:00, a flat plateau, and a step
  down at 10:00 the next day. The left edge tells you **when**, the height
  **how many**. The right edge is only the burst leaving the window, not a
  recovery. Values like 49.7 come from Prometheus's extrapolation; read them
  as 50.
- **Traffic is per second.** **HTTP traffic by route and status** is a
  five-minute rate. At Hooklook's volume the numbers are small: 0.0033 is one
  request in five minutes, 0.0167 is one a minute. The panel omits `static`
  and `sse`.
- **Quantiles are interpolated between bucket edges.** Request and SQLite
  duration buckets are 5, 10, 25, 50, 100, 250, 500 ms, 1, 2.5, 5 and 10 s. A
  p95 of 0.3 s means "between 250 and 500 ms". With only a few requests in five
  minutes, one slow request *is* the p95. A route with no requests in the
  window draws a gap, not a zero.
- **The latency panels include every route class,** also `health`, `static`
  and `other`. Hide the classes you do not care about by clicking the legend.
- **Storage and bin gauges are up to 30 seconds old.** The application
  recomputes them every 15 seconds and Prometheus scrapes every 15 seconds.
  When **Storage collection available** or **Active-bin collection available**
  is 0, the gauges beside it show their last good value and are stale.
- **Counters restart with the process.** `increase()` and `rate()` absorb the
  reset. After a restart a label combination (say `capture 507`) vanishes
  from the timeseries until it happens again; the top-row stats show 0
  because they fall back to `vector(0)`.
- **There is exactly one SQLite connection.** The pool is capped at one open
  connection, so every database call, including the 15-second telemetry
  collection and the once-a-second database health check, queues for it.
  **SQLite pool** therefore only ever shows 0 or 1, and **SQLite pool wait
  rate** is the interesting number: waiting seconds accumulated per elapsed
  second. 0.05 means that, taken together, callers spent 5% of wall time
  queuing.

## 2. Panel map

| Row / panel | Question it answers | Normal looks like |
| --- | --- | --- |
| **Overview**: Requests, Accepted captures, Missing-bin captures | How much happened in the selected range? | Captures a small share of requests |
| **Overview**: Application 5xx | Did the application fail anyone? `507` is excluded | 0, green |
| **Overview**: Service up | Can Prometheus scrape Hooklook right now? | 1, green |
| **Traffic**: HTTP traffic by route and status | Which kinds of request, with which outcome? | `health 200` (deploy checks, uptime probes), `capture 201`, `home 303`, `home 200` (cookieless crawlers and link previews), `bin_page 200`, bursts of `other 404` from scanners |
| **Traffic**: In flight HTTP requests | Is non-stream work piling up right now? | 0 |
| **Traffic**: HTTP request p50 / p95 | How long do requests take, per route class? | Tens of milliseconds or less |
| **Traffic**: HTTP traffic by traffic class | How much of the traffic is the generator? | A steady `synthetic` curve with a daytime peak; `other` bursty |
| **Bin activity**: Active bins | How many unexpired bins exist? | Roughly creations of the last three days, plus renewed bins |
| **Bin activity**: Active bins near limit | Is any bin at 450 requests or 90 MB or more? | 0 |
| **Bin activity**: Expired bins deleted, 24 hours | Is cleanup removing bins? | Close to the creations of three days ago |
| **Bin activity**: Open SSE streams | How many inspector tabs are live right now? | 0 when nobody is looking |
| **Bin activity**: Active-bin collection available | Is the bin count fresh? | 1 |
| **Bin activity**: Bin operations, 24 hours / 7 days | What are people doing? | `create` ≥ `list`; `capture` depends on who is testing |
| **Bin activity**: SSE events | How do streams start and end? | `open` ≈ `client_disconnect` + `channel_closed` |
| **Bin activity**: Capacity rejections by cause | Is anyone hitting limits? | Empty |
| **Capacity**: SQLite allocation and budget | How big is the database against `MAX_STORE`? | `allocated` far below `budget` |
| **Capacity**: SQLite budget used | The same as a percentage | Green, below 80% |
| **Capacity**: Data filesystem available | How much disk is left for the volume? | Comfortably more than `budget − allocated` |
| **Capacity**: Storage collection available | Are the storage numbers fresh? | 1 |
| **SQLite**: operation outcomes / errors | What does the application ask SQLite for, and how does it end? | `not_found` and `capacity` outcomes match the capture results; no errors |
| **SQLite**: operation duration | How long do database calls take? | Single-digit milliseconds |
| **SQLite**: pool, pool wait rate | Are callers queuing for the one connection? | Wait rate near 0 |
| **Expiry cleanup**: runs and failures | Is the minutely cleanup running? | About 1,440 `success`, no `error` |
| **Expiry cleanup**: Cleanup p95 duration | How long does one cleanup take? | The lowest bucket |
| **Runtime**: goroutines, memory, CPU, FDs | Is the process healthy? | Flat, rising a little with open streams |
| **Runtime**: Process uptime | When did the process last start? | Since the last full deployment |
| **Application logs** | What happened, line by line? | `http_request` lines, occasional `sse_*` and `cleanup_completed` |

## 3. The two-minute check

Do this when you have not looked for a while.

1. Keep **Last 24 hours**. Read the top row: **Service up** green,
   **Application 5xx** 0.
2. Check **Process uptime**. If it is shorter than the time since your last
   deployment, the process restarted on its own; go to
   [scenario 10](#scenario-10-the-process-keeps-restarting).
3. Glance at **SQLite budget used** and **Data filesystem available**.
4. Look at **Capacity rejections by cause** and **Active bins near limit**.
   Both should be empty or 0.
5. In **Expiry cleanup runs and failures**, the `error` series should not
   exist.
6. Scan **HTTP traffic by route and status** for a new colour: a status you
   have not seen before on a route you care about.

Write down your baseline the first few times: requests a day, captures a day,
bins created a day, active bins, allocated bytes. Hooklook's traffic is small
and bursty; without your own baseline, "a lot" means nothing.

## 4. Scenarios: product use

Each scenario starts from something you notice or wonder about, then walks the
panels in an order that narrows the answer.

### Scenario 1: real users, bots, or just me?

**Requests** says a few thousand for the day. How many of those are people?

1. Switch to **Hooklook non-synthetic traffic**, so the generator's bins and
   captures drop out, then open **HTTP traffic by route and status** and hover
   to read the series.
   Sort traffic into three piles:
   - **Machines checking on you:** `health 200` comes from deployment
     verification and any uptime monitor; `other 200` is `/ready`.
   - **Machines probing you:** `other 404` bursts are scanners trying paths
     Hooklook never had.
   - **Everything else** is product use: `home`, `bin_page`, `api_*`, `sse`,
     `capture`.
2. Among product use, compare `home 303` with `api_bin 200`. A new browser
   passes through `/` twice (the cookie check, then its new bin) and runs the
   frontend, which calls `api_bin` and `api_request_list`; a returning one
   passes through once. A crawler that drops cookies follows the first
   redirect, gets `home 200` (the cookies-required page) and stops, without a
   bin.

| Signature over the same window | Likely cause |
| --- | --- |
| `home 303` ≈ `bin_page 200` ≈ `api_bin 200`, with up to twice as many `home 303` when most visitors are new | People opening Hooklook in a browser |
| `home 303` ≈ `home 200`, little `bin_page` or `api_bin` | Crawlers that drop cookies, refused a bin (or link-preview bots, which add only `home 200`) |
| `home 303` ≈ `bin_page 404`, little `api_bin` | Clients that keep cookies across the check but not afterwards: rare, and each still creates a bin |
| `api_bin` and `api_request_list` well above `home` | People returning to bins, reloading, or reconnecting streams |
| `capture 201` without matching page traffic | Senders delivering webhooks while nobody watches |

For exact counts over a day, use Explore:

```promql
sort_desc(sum by (route, status) (increase(hooklook_http_requests_total[24h])))
```

Crawlers that drop cookies create no bins. The ones that keep them do, and such a bin costs little: a row in SQLite that expires after
three days. They inflate **Active bins** and the `create` count. How often a
bin was refused for missing cookies:

```promql
sum by (result) (increase(hooklook_bin_creation_results_total[24h]))
```

If you want the real number of bins that ever received a webhook, the
operator API lists every bin with its `requestCount` (see the [deployment runbook](deployment-runbook.md#operating-the-deployed-stack)
for the token-safe way to call it):

```sh
curl --fail -H "Authorization: Bearer $ADMIN_TOKEN" http://127.0.0.1:8081/admin/bins
```

### Scenario 2: do people come back?

A bin lives three days after its last use: owner visits, guest visits and
accepted captures all push its expiry forward. **Active bins** and the
creation rate together tell you how long bins actually live, by Little's law:
the number of things in a system equals the rate they arrive times how long
each stays.

1. Set the range to **Last 7 days**. Read **Active bins** (now) and the
   `create` value of **Bin operations, 7 days** at its right edge.
2. Compute `average lifetime = active bins ÷ (creates in 7 days ÷ 7)` in
   days.

| Result | Meaning |
| --- | --- |
| About 3 days | Almost nobody comes back. Bins die on their first expiry. |
| Clearly above 3 | Some bins are kept alive by repeat visits or ongoing captures |
| Below 3 | The creation rate rose recently; the population has not caught up |

Cross-check with **Expired bins deleted, 24 hours**. In a steady state it
approaches the daily creation rate of three days earlier. If it is much lower,
bins are being renewed.

Little's law only holds for a steady stream. Right after a crawler burst,
wait a week before trusting the number.

### Scenario 3: someone is sending webhooks into an expired bin

**Missing-bin captures** is not zero, and keeps growing.

Webhook senders do not know a bin has expired. A CI system, a payment sandbox
or a cron job keeps posting to the old URL and gets `404` forever.

1. Set the range to **Last 24 hours**. Open Explore and plot the rhythm:

   ```promql
   sum(increase(hooklook_capture_results_total{result="missing_bin"}[5m]))
   ```

   A flat line at, say, 5 per 5 minutes is one sender on a one-minute
   schedule. Bursts that come and go are retries: many senders back off
   exponentially after a failure.
2. Check the timing against **Expired bins deleted, 24 hours**. A
   missing-bin stream that started when bins were deleted is a sender whose
   bin just expired. One that predates any deletion is someone who mistyped
   or guessed a URL.
3. In **Application logs**, the matching lines are:

   ```logql
   {service="hooklook", route="capture"} | json | status = 404
   ```

   You will not find the bin code or the sender's address. That is by
   design. The logs tell you the rhythm and the method, which is often
   enough to recognise the sender.

Is it a problem? Each missing-bin capture costs one SQLite lookup and shows up
as `capture not_found` in **SQLite operation outcomes**, not as an error.
Caddy's capture limit of 60 a minute per IP puts a ceiling on one sender. It becomes worth acting on only if
it starts to dominate **SQLite pool wait rate**. There is no Hooklook-side
block list; Caddy owns that decision.

### Scenario 4: a capture burst. Can Hooklook keep up?

**Accepted captures** jumped: someone is replaying a queue of events, or load
testing their integration against a bin.

Put these side by side for the burst window: **HTTP traffic by route and
status** (`capture 201`), **SQLite operation duration** (`capture p95`),
**SQLite pool wait rate**, **SSE events** and **Active bins near limit**.

- **Capture p95 in SQLite stays in the low buckets and pool wait stays near 0:**
  Hooklook is idle between inserts. Nothing to do.
- **Pool wait rises:** captures queue for the single connection. A list or
  detail read on the same bin, the minutely cleanup and the telemetry
  collection all take the same connection. Look at **HTTP request p95** for
  `capture` to see how much of the queue reaches the sender.
- **`slow_subscriber` appears in SSE events:** an inspector tab could not keep
  up. The live stream holds one buffered event per tab. A tab that has not
  read it by the next capture is disconnected; the browser reconnects and
  refetches the list. This is the designed recovery, not a failure. Expect
  an `api_request_list` bump right after.
- **Active bins near limit goes to 1:** the bin crossed 450 requests or 90 MB.
  It will reject with `bin_full` soon (scenario 5).

Arithmetic worth knowing: Caddy allows one IP 60 captures a minute, at most 20
in any 20 seconds. One sender at full speed fills a 500-request bin in a bit
over eight minutes. A sustained rate above one capture a second in
**HTTP traffic** therefore needs more than one source IP.

### Scenario 5: a bin filled up, or the store filled up?

**Capacity rejections by cause** is no longer empty.

The two causes look alike from the sender's side, a `507`, but they are
completely different events:

| Cause | Scope | What the dashboard shows | What to do |
| --- | --- | --- | --- |
| `bin_full` | One bin reached 500 requests or 100 MB | **Active bins near limit** was 1 before it started; storage panels unchanged | Nothing. It is the product working. The owner can clear the bin. |
| `store_full` | SQLite reached `MAX_STORE`; every bin and every new visitor is affected | **SQLite budget used** at or above 100%; `home 507` in the traffic panel | Act now: see the capacity scenarios below |

Neither appears in **Application 5xx**, which excludes `507` on purpose. The
logs tell them apart too:

```logql
{service="hooklook", level="WARN"} | json | line_format "{{.msg}} {{.reason}}"
```

A single `store_full` is worth a look even if it stopped. Check which way it
was refused. `capture capacity` in **SQLite operation outcomes** means the
application's pre-check saw no room: the designed path. A `capture` error of
kind `full` in **SQLite errors** means the pre-check let the write through and
SQLite refused it at its hard cap, or the disk was full; go to
[scenario 6](#scenario-6-how-much-room-is-left), step 3.

## 5. Scenarios: capacity and health

### Scenario 6: how much room is left?

1. Open **SQLite allocation and budget**, range **Last 7 days**. Five lines:
   - `budget` is `MAX_STORE`, rounded down to whole pages. It moves only when
     the configuration changes.
   - `allocated` is the size of the main SQLite file in pages. It grows when
     captures arrive and rarely shrinks.
   - `reusable` counts free pages inside that file, left behind by deleted
     bins and requests. New data fills them before the file grows.
   - `main_file` is the file size the filesystem reports; it tracks
     `allocated`.
   - `wal` is the write-ahead log. It swells during bursts and shrinks at
     checkpoints.
2. The room that is really left is `budget − allocated + reusable`. **SQLite
   budget used** shows only `allocated ÷ budget`, so it overstates pressure
   after a big cleanup. Plot the real headroom in Explore:

   ```promql
   hooklook_storage_bytes{kind="budget"} - ignoring(kind) hooklook_storage_bytes{kind="allocated"} + ignoring(kind) hooklook_storage_bytes{kind="reusable"}
   ```

3. Compare with **Data filesystem available**. `MAX_STORE` limits only the
   SQLite file. Images, logs, the telemetry volumes, backups and zibs share
   the same disk. **If filesystem available is smaller than the headroom from
   step 2, the disk runs out before `MAX_STORE` does**, and SQLite fails with
   a real `full` error in **SQLite errors** instead of a clean pre-check
   rejection. The capture result is `store_full` either way. The deploy-time
   [storage headroom check](storage-capacity.md) guards this at deployment,
   not in between.
4. For a trend, ask Prometheus when allocation would reach the budget if the
   last week's growth continued:

   ```promql
   (hooklook_storage_bytes{kind="budget"} - ignoring(kind) hooklook_storage_bytes{kind="allocated"})
   / ignoring(kind) deriv(hooklook_storage_bytes{kind="allocated"}[7d]) / 86400
   ```

   The answer is in days. A negative or huge value means allocation is not
   growing: new captures are filling freed pages.

Patterns you may see:

| Pattern | Meaning |
| --- | --- |
| `allocated` steps up, never down; `reusable` rises and falls | Healthy: expired bins free pages, new captures reuse them |
| `allocated` flat while `reusable` falls towards 0 | The file is about to start growing again |
| `wal` stays large for hours | A long read kept checkpoints from finishing. Rare with one connection. |
| `main_file` far above `allocated` | Not expected; check the collection panels and the logs |

### Scenario 7: the inspector feels slow

The owner reports that opening a bin or clicking a request takes a while.

1. Open **HTTP request p95** and hide everything but `bin_page`, `api_bin`,
   `api_request_list` and `api_request_detail`. Is one of them high, or all?
2. Compare with **SQLite operation duration** for `list` and `detail`.

| You see | Meaning | Next step |
| --- | --- | --- |
| HTTP p95 high, SQLite p95 low, pool wait near 0 | Time goes outside the database: large JSON bodies, CPU | **Process CPU rate**; request detail for big bodies is inherently slower |
| HTTP p95 and SQLite p95 high together | The queries themselves are slow | A bin near 500 requests makes `list` do more work; check **Active bins near limit** |
| SQLite p95 fine, pool wait rising | Calls queue for the one connection | Find the other users of the connection: a capture burst (scenario 4), cleanup (**Cleanup p95 duration**) |
| **Active-bin collection available** flickering to 0 | The telemetry query, which counts requests for every bin, exceeded its 750 ms timeout | The bin population has grown enough to make that query expensive; it holds the only connection while it runs |

The frontend's own feeling of slowness includes the network and Caddy.
Caddy's **p95 time to first byte** for `hooklook.app` shows the same request
from the outside. If Caddy's number is much higher than Hooklook's, the time is
spent between them or on the host.

### Scenario 8: live updates stopped arriving

An owner says new captures only appear after a reload.

1. Check **Open SSE streams**. With the owner's tab open it should be at least
   1. If it is 0, the tab has no stream.
2. Read **SSE events** for the window. The close reasons:

| Event | Meaning |
| --- | --- |
| `open` | A tab subscribed to a bin |
| `client_disconnect` | The tab closed or navigated away. The normal ending. |
| `channel_closed` | Hooklook ended the stream: the bin expired, sharing was turned off for a guest, the tab was a `slow_subscriber`, or the process shut down |
| `slow_subscriber` | A tab missed an event during a burst; its stream then ends as `channel_closed` |
| `write_failure`, `flush_failure` | Writing to the connection failed; the client or Caddy went away mid-event |
| `invariant_failure` | A captured request could not be encoded. Should never happen; the stream ends as `encode_failure`. |

3. A stream that opens, closes and reopens every few seconds shows as `open`
   and `write_failure` climbing together. Something is cutting idle
   connections. Hooklook sets no write deadline on streams, so look between
   the browser and the server: Caddy's logs, a corporate proxy, a browser
   extension.

A leak check: every stream that opened either closed or is still open. Over
any window,

```promql
sum(increase(hooklook_sse_events_total{event="open"}[1h]))
- sum(increase(hooklook_sse_events_total{event!~"open|slow_subscriber|invariant_failure"}[1h]))
```

should equal the change in **Open SSE streams** over the same hour, give or
take a stream opening during the scrape. `slow_subscriber` and
`invariant_failure` are excluded because each is followed by its own close
reason. A result that keeps growing while tabs are closed means streams are
not being released.

### Scenario 9: after a deployment

A push to `main` deployed. What should the dashboard show?

It depends on the mode the plan chose (see the
[deployment runbook](deployment-runbook.md#how-a-push-deploys)):

| Mode | What you see |
| --- | --- |
| `dashboard` | Nothing changes in the data; only the dashboard layout |
| `observability` | A gap of one or two scrapes in every panel while Prometheus restarts. **Process uptime** keeps counting. |
| `full` | Everything below |

In full mode:

1. **Process uptime** drops to zero. That is your timestamp.
2. Every counter restarts. Rolling-window panels dip and recover smoothly,
   because `increase()` absorbs the reset.
3. Just before the restart, every open stream ends as `channel_closed`. The
   old process exits before Prometheus scrapes again, so those final counts
   are lost; the `sse_closed` log lines are not. Within seconds, **Open SSE
   streams** climbs back as tabs reconnect, and `api_request_list` gets a
   small burst from their refetch.
4. The deployment's own checks show up: `health 200` and `other 200`
   (`/ready`) on the loopback, then one public `health 200` through Caddy.
5. **Application logs** shows `service_started`.

The same goes for anything else in the old process's last seconds. A stream
that tried to open during shutdown got `503`, but you will find it only in the
logs:

```logql
{service="hooklook", route="sse"} | json | status = 503
```

Hooklook is recreated in place, so Caddy returns `502` for the few seconds in
between. Hooklook cannot see those; Caddy's **502 and 504** panel can.

### Scenario 10: the process keeps restarting

**Process uptime** is shorter than it should be, or **Service up** flickers.

Hooklook deliberately exits when its database becomes unusable. It checks
SQLite every second, and a failed expiry cleanup also stops it. Docker then
restarts the container (`restart: unless-stopped`). The dashboard shows each
restart as an uptime reset; the metrics from the last seconds before the exit
are usually lost, because the process died before Prometheus scraped them.

1. Count restarts in Explore:

   ```promql
   changes(process_start_time_seconds{job="hooklook"}[24h])
   ```

2. The logs survive, because Alloy reads Docker's log files. Find the last
   words of each run:

   ```logql
   {service="hooklook", level="ERROR"}
   ```

   `cleanup_failed` (with an `error_class`) and `server stopped with error`
   (with the cause, such as `database became unavailable`) are the ones to
   expect.
3. `error_class="full"` means SQLite could not write: the disk or the budget.
   Go straight to [scenario 6](#scenario-6-how-much-room-is-left).
4. On the VPS, `./scripts/compose.sh ps` and
   `./scripts/compose.sh logs --tail=100 hooklook` in `/opt/hooklook` show the
   restart loop directly.

| **Service up** | **Process uptime** | Meaning |
| --- | --- | --- |
| 1 | Resets now and then | Hooklook exits and Docker restarts it; read the ERROR logs |
| 0 | No data: its series goes stale as soon as a scrape fails | Hooklook is down, or Prometheus cannot reach `hooklook:9092` |
| No data | No data | Prometheus itself is down; the logs panel still works |

### Scenario 11: Application 5xx turned red

1. Set the range to **Last 1 hour**, then find the route with Explore, since
   the traffic panel hides `sse`:

   ```promql
   sum by (route, method, status) (increase(hooklook_http_requests_total{status=~"5..", status!="507"}[1h]))
   ```

2. Use the table in section 1:
   - `capture 500` is an `internal_error` capture result: the insert failed
     for a reason other than capacity. **SQLite errors** has the kind.
   - `sse 503` is a stream that tried to open while the process shut down.
     It rarely reaches the metrics (scenario 9).
   - `home`, `bin_page`, or `api_* 500` means a SQLite lookup failed.
3. Read the logs for the same minutes:

   ```logql
   {service="hooklook"} | json | status >= 500
   ```

A burst of 500s followed by an uptime reset is scenario 10 seen from the
other side: requests failed while the database was going away, and then the
process stopped.

## 6. Log recipes

Open **Explore**, choose the **Hooklook Loki** data source, and paste a query.
Loki keeps seven days. Three labels are indexed: `service` (always
`hooklook`), `level` (`INFO`, `WARN`, `ERROR`) and `route` (only on
`http_request` lines). Everything else is a JSON field that `| json` extracts:
`msg`, `method`, `status`, `traffic_class`, `duration_seconds`, `reason`,
`error_class`, `bins_deleted`.

Everything that is not a plain request:

```logql
{service="hooklook"} | json | msg != "http_request"
```

Slow requests, with their route:

```logql
{service="hooklook"} | json | msg = "http_request" and duration_seconds > 0.5
```

Status mix per route over five-minute steps, as a graph:

```logql
sum by (route, status) (count_over_time({service="hooklook", route=~".+"} | json [5m]))
```

Why streams ended in the last day:

```logql
sum by (reason) (count_over_time({service="hooklook"} | json | msg = "sse_closed" [24h]))
```

How many bins cleanup removed, run by run:

```logql
{service="hooklook"} | json | msg = "cleanup_completed" | line_format "{{.bins_deleted}}"
```

Static assets are not logged, so a page load is three or four lines, not
twenty.

## 7. Explore recipes

Open **Explore** and choose the **Hooklook Prometheus** data source. Prometheus
keeps 14 days, twice as long as Loki.

Real database failures on captures (the result should be 0). Missing bins and
capacity rejections are outcomes, not errors, so nothing needs subtracting:

```promql
sum by (kind) (increase(hooklook_db_errors_total{operation="capture"}[24h]))
```

The same captures seen from both sides. `not_found` should equal
`missing_bin`, and `capacity` should equal `bin_full` plus `store_full`,
unless SQLite's own cap was hit (then the difference is in the errors above):

```promql
sum by (result) (increase(hooklook_db_operations_total{operation="capture"}[24h]))
```

Share of captures accepted, per day:

```promql
sum(increase(hooklook_capture_results_total{result="accepted"}[24h]))
/ sum(increase(hooklook_capture_results_total[24h]))
```

Bin creation attempts from `/`: `created`, `no_cookie` (the client did not
return the cookie check) and `store_full`:

```promql
sum by (result) (increase(hooklook_bin_creation_results_total[24h]))
```

Which methods webhooks use (Hooklook accepts any):

```promql
sum by (method) (increase(hooklook_http_requests_total{route="capture"}[7d]))
```

Share of API reads answered within 50 ms, a latency target that a handful of
slow requests cannot distort:

```promql
sum(rate(hooklook_http_request_duration_seconds_bucket{route=~"api_.*", le="0.05"}[1h]))
/ sum(rate(hooklook_http_request_duration_seconds_count{route=~"api_.*"}[1h]))
```

Average SQLite time per operation, instead of the p95:

```promql
sum by (operation) (rate(hooklook_db_operation_duration_seconds_sum[1h]))
/ sum by (operation) (rate(hooklook_db_operation_duration_seconds_count[1h]))
```

Did cleanup miss any minute? Expect about 60 an hour:

```promql
sum(increase(hooklook_expiry_cleanup_runs_total[1h]))
```

## 8. Practice drills

These are harmless: a handful of requests, far below any limit. Do not
practise capacity or rate-limit rejections against production; the
[production verification runbook](production-verification-runbook.md)
describes the deliberate checks and when to run them.

Open `https://hooklook.app` in a browser first, keep the bin page open, and
note its code.

1. **Watch your own capture arrive.** Send one webhook:

   ```sh
   curl -sS -o /dev/null -w '%{http_code}\n' -X POST \
     -H 'Content-Type: application/json' -d '{"drill":1}' \
     https://hooklook.app/b/YOUR_BIN_CODE
   ```

   It prints `201`. Set the range to **Last 15 minutes**. Within a minute,
   **Accepted captures** rises by one, `capture 201` appears in the traffic
   panel, and the logs show one `http_request` line with `route=capture`.
   **Open SSE streams** stays the same: the capture went down the existing
   stream.
2. **Tell an outcome from an error.** Send the same request to a code that
   does not exist, such as `https://hooklook.app/b/drill-missing`. It prints
   `404`. **Missing-bin captures** rises by one, and so does
   `capture not_found` in **SQLite operation outcomes**. **SQLite errors**
   does not move: the lookup worked, it just found no bin. Now run the first
   query in section 7: it stays 0.
3. **Follow a stream's life.** Note **Open SSE streams**, open the same bin in
   a second tab, and watch it rise by one. `sse` does not appear in the
   traffic counter yet. Close the tab: the gauge drops, **SSE events** gains a
   `client_disconnect`, and only now does the request get counted.
4. **Find your page load in the logs.** Reload the bin page, then query
   `{service="hooklook", route=~"bin_page|api_bin|api_request_list"}`. The
   three lines of one page load carry the same second.
5. **Compute bin lifetime** with the scenario 2 formula, and write it down.
   Repeat in a month.

## 9. When to leave this dashboard

| Question | Where |
| --- | --- |
| Were requests rejected with `413`, `429` or `431` before reaching Hooklook? | Caddy dashboard in hetzner-one's Grafana; see its [dashboard guide](https://github.com/dvinubius/hetzner-one/blob/main/docs/observability-guide.md) |
| Did the public site return `502` while Hooklook restarted? | The same Caddy dashboard, **502 and 504 responses** |
| Which bin, which sender, which IP? | Deliberately not recorded. `GET /admin/bins` lists bins with their request counts. |
| What is the exact storage state right now? | `GET /admin/storage` (see the [deployment runbook](deployment-runbook.md#operating-the-deployed-stack)) |
| Is the whole VM short of CPU, memory or disk? | hetzner-one's Host dashboard; Hooklook has no host metrics |
| Is `hooklook.app` reachable from the internet? | `curl https://hooklook.app/health` from your workstation, or the [production verification runbook](production-verification-runbook.md) |
| Did the telemetry stack itself break? | `./scripts/telemetry-smoke-test.sh` on the VPS, and the [observability runbook](observability-runbook.md) |
