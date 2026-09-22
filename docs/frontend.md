# Frontend

The inspector is one Vue 3 application, served by Go on the two authorized page
routes and built by Vite into `frontend/dist`, which the binary embeds. There is
no router, no store and no frontend backend: the page is about exactly one bin,
and the server is the authority on everything private.

The visual language is not decided here — see
[visual style](../.agents/VISUAL_STYLE.md) — and the wire contract is the
[HTTP API](http-api.md).

## Where it runs

Go serves the same document for `/bins/{code}` and for
`/bins/{code}/requests/{id}`, the detail URL a capture reports, after the same
authorization. The application resolves the request id itself, so an unknown id
is not a server error. Because the document references its assets by absolute
path, a direct navigation or a reload works at either depth.

The page is laid out for viewports 1024px wide and up, and has no media
queries: narrower screens are not supported.

In development the browser stays on the **Vite** origin, and Vite proxies the
home, page, API, SSE and capture routes back to Go. One origin is what makes the
`HttpOnly` owner cookie, Go's `Origin` check on mutations, and `EventSource` all
agree — while Go still authorizes every page before Vite's modules load.
`PUBLIC_BASE_URL` must name that origin.

When the store cannot create a bin for a first-time visitor, `/` responds with
`507` and still serves the application document. The document's `<html>` has
`data-hooklook-startup="store_full"`; the browser reads that marker before
attempting a bin session and puts the session straight into `store_full`,
without calling the API — there is no cookie, no code and nothing to authorize.
That state renders `ServiceFull`: the page's own bars, and an apology centred
between them. It offers no retry control, because a visitor cannot free the
space and a button that reloads into the same wall reads worse than the
sentence; a reload is still the way back in once room exists.

Unavailable bin documents use parallel startup markers. An unavailable regular
URL uses `bin_expired`; an unavailable URL with a nonempty `invite` parameter
uses `shared_bin_unavailable`. The link shape controls the message for missing,
expired, invalid, and revoked targets alike. The session makes no metadata
request for either marked document and renders
`BinUnavailable` inside the shared top bar and footer. Its centred `//` aside
explains the applicable state, with **Create New Bin** underneath linking to
`/`. An API refusal discovered after load changes to the same empty state. The
wordmark in every shared top bar also links to `/`.

## Authorization is never assumed

Go authorized the document before the browser ran a line of this, but the page
must not render private data on that basis alone: a bin expires, an invitation
is revoked, a restored back/forward page outlives both. So `lib/session.ts`
calls `GET /api/bins/{code}` before anything private renders and treats that
response as the **only** source of role and sharing state.

Failures split in two:

- **`403`/`404` — the target is unavailable.** Everything in flight stops and
  the page shows the corresponding framed empty state. No replacement exists
  until the visitor chooses **Create New Bin**.
- **Network or `5xx` — not right now.** Recoverable, so the page offers a retry
  and never redirects.

A metadata response that wins the race against its own cancellation is
discarded rather than rendered.

The invitation lives in this module's memory and on same-origin request URLs
only. It is never stored, logged or handed to anything third-party, and
`inviteId` is rendered nowhere except as the owner's own share link. The owner
cookie is `HttpOnly` and is never read at all.

## The request list

`lib/feed.ts` keeps the list current from two sources at once: an `EventSource`
stream, and a fetch of the persisted summaries issued immediately alongside it.

**SQLite is the truth; the stream only says when to read it.** The server
replays nothing and disconnects a subscriber it could not hand an event to, so
the feed refetches the whole list on a `refresh` event, after every reconnect,
and when a disconnected tab returns to the foreground. A tab that was asleep
converges rather than drifting.

**Events are merged by request id, never appended.** Events seen since the
newest fetch was *issued* are kept when that fetch lands, because they are newer
than the snapshot it answers with. That is what stops an event racing a fetch
from either duplicating a row or losing a capture — and it is why a refetch can
still drop rows a deletion removed.

**Revoked access ends, rather than retrying forever.** `EventSource` reconnects
indefinitely by design, so a stream failure triggers exactly one authorization
recheck per disconnection episode; if the answer is that access is gone, the
stream closes and the session leaves. A network failure answers "unknown", which
is not "revoked", so an offline tab keeps retrying. A summaries fetch that is
itself refused needs no recheck — that request *was* the check.

`lib/list.ts` arranges what the feed holds, locally:

- **Order** is receipt time, with the numeric row id as tie-breaker. Receipt
  time is truncated to the second, so ties are the common case, not an edge one;
  the id keeps the order stable as live events arrive.
- **Filters** are an exact method (chosen from the methods the bin has actually
  seen), and a text filter each for path and raw query. A text filter has an
  operator — **matches** (a case-insensitive substring, typed in a field that
  only this operator shows), **is empty** or **is not empty**. A path of `/`
  counts as empty. Filtering is a view concern only — a filtered-out row is
  still in the feed.

While a filter is on, the list says what it left — `Results: n (Total: m)`,
with `n` at the reading tier because it is the number that moves — between the
controls and the rows, with a hairline above and below it. An
unfiltered list says no number at all: what is on screen is what there is, and
a total that only ever agrees with the rows under it is a line to read for
nothing.

Loading, empty, filtered-empty and recoverable-error are each distinct states
named in words. The brand defines no motion, so the stream's state is said
rather than pulsed: "streaming" beside a still green dot while it is live, and
"connecting…" beside a muted grey one otherwise — a first connection, a
reconnection and a closed stream read the same. The dot is the same shape
either way, so the row never changes width as the state changes; only the word
and the colour differ, and the word is what carries the meaning.

## How full the bin is

The capture row carries a **capacity gauge** at its right end, before the sweep
button, on a code surface as high as the capture link beside it — the row is a
set of blocks of one height, and a gauge floating on the page between two of
them read as something the row had forgotten to frame. It sits with the
controls that act on the bin rather than with the list,
because it is a fact about the bin itself — the list can be filtered down to one
row and the bin still be out of room. A guest sees none of it: the whole row end
is the owner's, and a reader who cannot make room has no use for the number —
see the [README](../README.md) for who a guest typically is and why their page
is the requests and nothing else.

A bin has two limits — 100 MB of raw bodies and 500 requests — and a capture is
refused as soon as either is reached. They run down at their own rates, so the
gauge is **two readings side by side**, `storage` then `requests`, each with its
own bar and percentage: a bin can be out of bytes with four hundred slots to
spare, and one number folding the two together would hide that. Each reading floors, and stops
at 99 until the server's own flag for *that* limit is set, so "100%" is never
shown beside a limit that still admits a capture. A bar is green up to 90% and
brick past it, the same brick the destructive controls use, and only the row
that is running out turns. `lib/capacity.ts` holds the arithmetic as pure
functions; each row's title spells its limit out in full.

Capacity is not in the SSE payloads — a summary says what arrived, not how much
room is left — so `BinPage` re-reads `GET /api/bins/{code}` when the list
changes, coalescing a burst of captures into one re-read half a second later.
The number is a status line, not a counter, and the server is what enforces the
limit in any case.

Global capacity is a separate fact on the same response, and it is not the
gauge's to report: that bin may be nearly empty and still take nothing, because
every bin shares one store. A bin page whose `storeCapacity.full` is set says so
once, at the top, in brick — the only line on the page that is neither the
visitor's doing nor theirs to fix, and it governs whether anything arrives at
all.

## One request in full

The list stays body-free, so selecting a row fetches that request's detail on
its own. The selection lives in the **URL**, which is what makes back, forward
and a capture's own detail link select the same thing; `history.pushState`
records a click, `replaceState` is used when clearing a selection that no
longer exists so the history does not gain a dead entry. Up and down arrows move
the selection through the list as shown, also with `replaceState`, so walking
the list does not leave one history entry per row.

A selection that changes while a detail request is in flight aborts it, and a
response that arrives anyway is discarded by sequence number. A `404` on a
detail fetch means *this request* is not in the bin — a stale link, a deleted
row — and is reported as such; it never invalidates the session, because the
list fetch and the stream are what discover revoked access. A selection the
refreshed list no longer contains is reported the same way instead of leaving
stale detail on screen. That note sits in the middle of the pane rather than at
its top, on a fill no wider than the line itself: there is no request under it
for it to head.

While a detail is in flight the pane does not empty. The request already on
screen stays, under a veil with a spinner at its centre, until the next one
arrives; the veil sits over the pane rather than inside the scroller, so it
stays centred however far the reader has scrolled. A pane that emptied to a
line of text flickered on every selection, and the words said nothing the
spinner does not — the spinner names itself to screen readers instead. What
the pane keeps is content, not position: a new request opens at its own top.
A missing or failed request replaces the pane as before; only the wait is
veiled.

### Client address

The facts line above the headers reports a **client ip** when the capture
carried an `X-Forwarded-For`, read off the stored headers by `lib/headers.ts`.
Nothing about it is stored: there is no column and no capture field for an
address, only this reading of a header that was captured like any other.

The header is looked up case-insensitively, and a repeated header and a
comma-separated one flatten to the same list of hops, as HTTP treats them.
The reported address is the **last** hop, not the first. Caddy appends the
address it accepted the connection from, so a caller who sends
`X-Forwarded-For: 1.2.3.4` arrives as `1.2.3.4, <their real address>`: the
first hop is whatever the caller claimed and the last is the only one this
deployment observed. Ordinary traffic has one hop and the two readings agree.

This depends on Caddy being the only public ingress, with Go unreachable
directly. **Add another proxy or a CDN and the trusted hop moves further
left** — `clientIp` in `lib/headers.ts` is where that rule lives. A capture
that never passed the proxy has no header and no client ip line.

### Bodies

`lib/body.ts` turns `rawBody` back into the exact bytes Go stored, then into
something readable, in this order:

1. **Decode** the base64 to a `Uint8Array`. An absent body arrives as an empty
   string, not `null`, and is reported as *empty* rather than as empty text.
2. **Binary or text.** A NUL byte is decisive; otherwise it takes more than 5%
   of C0 controls, so one stray byte in a log line does not turn a readable body
   into a hex dump.
3. **Decode text** by trying the declared `charset`, then UTF-8, then
   `windows-1252` — which maps every byte and so always succeeds. The encoding
   that won is named on screen, and a fallback is stated rather than hidden: it
   is the honest "these bytes are not UTF-8", not a guess dressed as a fact.
4. **Format** JSON and XML, chosen by content type or sniffed from the first
   character. Both formatters are dependency-free; the XML one walks the text
   and prints it back, and refuses input it cannot account for (an unclosed or
   mismatched element) rather than repairing it. Formatting failure keeps the
   raw view and explains itself locally. Bodies over 256 KiB skip formatting —
   and highlighting with it, since both walk the whole string — so they are
   shown raw and unhighlighted rather than truncated. Nothing trims a text
   body: only the binary dump has a cap.
5. **Highlight** into tokens carrying a role — `name` (Violet) for JSON keys
   and XML element and attribute names, `value` (Teal) for JSON strings,
   numbers and literals and XML attribute values and text, `recede` for
   punctuation, comments and declarations.

Binary bodies get a hex dump — offset, sixteen bytes, printable ASCII — capped
at 2 KiB with the truncation stated.

Every body reads at 12px, a step below the code surface's own 13px, whichever
form it takes: formatted, raw, or a dump row of three fixed columns that a
narrow pane would otherwise cut. The capture link at the top of the page and
the invitation link in the share popover keep their own sizes.

### Captured content never becomes markup

This is the property the whole pipeline exists to protect. Highlighting produces
**token data**, and components render it through text interpolation; nothing
uses `v-html`, and captured bytes are never handed to a DOM parser — which is
also why the XML formatter is hand-written. A body containing `<script>` is
characters on a page. `src/tests/render.test.ts` asserts exactly this against
real rendered output.

### Headers

Shown as stored, sorted by name, as a table on the page — not a code surface —
with one header per row between hairlines. The name column is a fixed width, so
a long name is cut with an ellipsis and shown whole in a popover while hovered;
the whole name is in the DOM either way, so a screen reader hears it regardless.
A hovered row is shaded and lays a copy control over the right end of its value,
which copies the whole header as `Name: value`, several values folded with
`", "`.

Values the server replaced with `[REDACTED]` before writing them down are shown
as stored, and an info note by the section's label says they cannot be recovered
here — there is nothing that tries. The body's own note says the opposite about
bodies: they are stored exactly as received.

## Owner actions, and what a guest is

Owners get sharing on/off, the invitation link, delete one request, and clear all
requests. Capability comes from `owner` in the metadata response. The bin's top
row carries a help button beside the capture link, and the capacity gauge, a
sweep button and a share button, in that order, at its right end — of which a
guest sees the gauge alone. Deleting one request is an icon at the right
end of the selected row in the list. The selection is left where it was, so the
detail pane reports the request as no longer in the bin — the same note a stale
link or another tab's clear produces — rather than moving the reader to a
request they did not ask for.

The share button opens a popover under it, right edges aligned, holding the
**Guest access** switch and the invitation link with its copy control. It is a
native popover: it closes on Esc, on a click elsewhere, or when the window is
resized. The switch flips once the server has saved the change, not on the
click.

The help button opens a dialog with five sections: an example request against
the capture URL, what the bin holds and what to do when it is full, what
redaction does to credential headers, how sharing works and that the invitation
link only works while guest access is on, and how long a bin is kept. It does not hold the link itself. The sweep button opens a confirmation that names what
clearing destroys; a failure is reported in the dialog, which stays open. Both
are native `<dialog>`s that close on their ×, on Esc, or on a click on the
backdrop; their contents are unmounted while closed, so a half-confirmed action
does not survive closing one. **Clearing keeps the bin** — same capture URL and
same invitation.

Disabling sharing does not change the invitation link; it stops the link
working, and closes the streams guests hold on it. The owner's own stream is
left connected — they asked for this from their live page, and that page does
not have to reconnect. Enabling it again makes the same link work.

**Hiding the owner controls from a guest is presentation only.** The server
re-checks ownership, the cookie and the request `Origin` on every mutation, so a
guest calling those endpoints directly is refused there — `403` — not here.
Each kind of change — saving access, clearing, deleting one request — is pending
on its own, so one never disables the controls of another; a repeat click on the
same kind while it is pending is ignored. Mutation failures are reported by kind
(`403`, `404`, other status, unreachable) and never claim a success that did not
happen.

## Modules

| Path | Responsibility |
| --- | --- |
| `api.ts` | The only place that talks to Go. Same-origin, invitation on the URL, `ApiError` with an `unauthorized` predicate. |
| `lib/session.ts` | Bootstrap, role, the two failure classes, the recovery loop guard, and one `AbortController` for everything dependent. |
| `lib/feed.ts` | Stream plus summaries, id reconciliation, refetch triggers, revocation recheck. Its browser wiring is separable from its logic. |
| `lib/list.ts` | Ordering and filtering. Pure functions. |
| `lib/capacity.ts` | How full a bin is as one percentage, the warning threshold, and the wording of the gauge's title. Pure functions. |
| `lib/body.ts` | Base64 → bytes → text → formatted → tokens, plus the hex dump. Pure functions. |
| `lib/location.ts` | Bin code, request id and invitation read from the URL; capture and invitation URLs built on the origin the browser is really on. |
| `lib/headers.ts` | The client address read out of `X-Forwarded-For`, and the trust rule that picks which hop. Pure functions. |
| `lib/format.ts`, `lib/clipboard.ts`, `lib/theme.ts` | Times and byte counts, a clipboard that is allowed to be unavailable, a dark-by-default theme toggle. |
| `components/` | `BinPage` wires the feed, selection, detail and mutations; the rest render. `PageShell` is the shared top bar and footer, `ServiceFull` the out-of-room page, `BinUnavailable` the expired and inaccessible shared-bin page, and `CapacityGauge` the bin's room on the capture row. `ThemeToggle` mirrors its zibs counterpart. |

## Tests

`npm test` runs the suite in Node — no browser, no DOM package. Component tests
render through Vue's own server renderer, which ships with Vue, and that is what
checks captured content is escaped and that a guest is rendered no control
they cannot use. `src/tests/captures.test.ts` decodes `rawBody` values copied
out of a running server, so the seam between the two halves is asserted against
what the server really sends rather than a hand-written fixture.

Browser-visible behavior is reviewed in a browser by hand.
`scripts/exercise-inspection.sh` covers the server side of the same flow.
