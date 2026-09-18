/** The live request list: an SSE stream plus the persisted summaries.
 *
 * The stream is a notification channel, not a log. It replays nothing, and a
 * subscriber the server could not hand an event to is disconnected on purpose.
 * So SQLite is always the source of truth and the stream only says *when* to
 * read it: the feed fetches the list immediately, refetches on a `refresh`
 * event, after every reconnect, and when a disconnected page returns to the
 * foreground. Events are merged by request id, never appended, so an event
 * racing a fetch can neither duplicate a row nor lose a capture.
 *
 * A guest's access can be revoked mid-stream, and `EventSource` would retry
 * that forever. After a stream failure the feed asks the server once whether
 * this visitor may still read the bin, and leaves the page when the answer is
 * no. */

import { ref, type Ref } from 'vue'
import { api, ApiError, eventsUrl } from '../api'
import type { RequestSummary, StreamState } from '../types'

export interface StreamHandlers {
  /** The stream is connected. Fired again after every automatic reconnect. */
  open(): void
  request(summary: RequestSummary): void
  refresh(): void
  failed(): void
}

export interface StreamHandle {
  close(): void
}

export interface FeedEnvironment {
  fetchSummaries(signal: AbortSignal): Promise<RequestSummary[]>
  openStream(handlers: StreamHandlers): StreamHandle
  /** True unless the server said outright that this visitor is not allowed.
   *  A network failure answers true: "unknown" is not "revoked". */
  stillAuthorized(): Promise<boolean>
  /** Called once when authorization is gone. The session leaves from here. */
  revoked(): void
  /** True when a failed fetch was a refusal rather than a bad moment. */
  invalidates(cause: unknown): boolean
  signal: AbortSignal
  visible(): boolean
  onVisibilityChange(listener: () => void): () => void
}

export interface RequestFeed {
  /** Store order. Sorting and filtering are the reader's, and happen above. */
  readonly summaries: Ref<RequestSummary[]>
  readonly stream: Ref<StreamState>
  /** A first list is still on its way; the page has nothing to show yet. */
  readonly loading: Ref<boolean>
  /** A list has arrived at least once, so empty really means empty. */
  readonly loaded: Ref<boolean>
  /** A recoverable fetch failure, in words, or empty. */
  readonly error: Ref<string>
  start(): void
  refetch(): Promise<void>
  /** Drops one row locally after the owner deleted it, so the list does not
   *  show it while the confirming refetch is still in flight. */
  forget(id: string): void
  stop(): void
}

export function createFeed(env: FeedEnvironment): RequestFeed {
  const rows = new Map<string, RequestSummary>()
  // Events seen since the newest fetch was issued. They are newer than the
  // snapshot that fetch will answer with, so they survive it.
  const sinceFetch = new Map<string, RequestSummary>()

  const summaries = ref<RequestSummary[]>([])
  const stream = ref<StreamState>('connecting')
  const loading = ref(true)
  const loaded = ref(false)
  const error = ref('')

  let handle: StreamHandle | null = null
  let dropVisibilityListener: () => void = () => {}
  let latestFetch = 0
  let rechecking = false
  let stopped = false

  function publish(): void {
    summaries.value = [...rows.values()]
  }

  async function recheck(): Promise<void> {
    // One question per disconnection episode: `EventSource` reports an error
    // on every retry, and the answer does not change between two of them.
    if (rechecking || stopped) return
    rechecking = true
    try {
      if (!(await env.stillAuthorized())) {
        stop()
        env.revoked()
      }
    } finally {
      rechecking = false
    }
  }

  async function refetch(): Promise<void> {
    if (stopped) return
    const attempt = ++latestFetch
    sinceFetch.clear()
    if (!loaded.value) loading.value = true
    try {
      const fetched = await env.fetchSummaries(env.signal)
      if (stopped || attempt !== latestFetch) return
      rows.clear()
      for (const row of fetched) rows.set(row.id, row)
      for (const [id, row] of sinceFetch) rows.set(id, row)
      sinceFetch.clear()
      publish()
      loaded.value = true
      error.value = ''
    } catch (cause) {
      if (stopped || attempt !== latestFetch) return
      if (env.invalidates(cause)) {
        // The list request is itself an authorization check, so there is
        // nothing left to ask.
        stop()
        env.revoked()
        return
      }
      error.value = 'The request list could not be loaded just now.'
    } finally {
      if (!stopped && attempt === latestFetch) loading.value = false
    }
  }

  const handlers: StreamHandlers = {
    open() {
      if (stopped) return
      const reconnected = stream.value === 'reconnecting'
      stream.value = 'live'
      // Whatever arrived while the stream was down is in SQLite, not in the
      // stream — read it rather than wait for an event that will not come.
      if (reconnected) void refetch()
    },
    request(summary) {
      if (stopped) return
      rows.set(summary.id, summary)
      sinceFetch.set(summary.id, summary)
      publish()
    },
    refresh() {
      if (stopped) return
      void refetch()
    },
    failed() {
      if (stopped) return
      stream.value = 'reconnecting'
      void recheck()
    },
  }

  function onVisibilityChange(): void {
    if (stopped || !env.visible()) return
    // A foregrounded page whose stream is not live has missed an unknown
    // amount; a live one has not.
    if (stream.value !== 'live' || !loaded.value) void refetch()
  }

  function start(): void {
    if (stopped) return
    handle = env.openStream(handlers)
    dropVisibilityListener = env.onVisibilityChange(onVisibilityChange)
    void refetch()
  }

  function stop(): void {
    if (stopped) return
    stopped = true
    stream.value = 'closed'
    handle?.close()
    handle = null
    dropVisibilityListener()
    dropVisibilityListener = () => {}
  }

  function forget(id: string): void {
    rows.delete(id)
    sinceFetch.delete(id)
    publish()
  }

  return { summaries, stream, loading, loaded, error, start, refetch, forget, stop }
}

/** The browser wiring: `EventSource` for the stream, the API module for the
 *  list, and the document's own visibility. Same-origin requests carry the
 *  owner cookie on their own; a guest's invitation rides on the URL. */
export function browserFeedEnvironment(
  code: string,
  invite: string,
  signal: AbortSignal,
  revoked: () => void,
): FeedEnvironment {
  return {
    signal,
    revoked,
    invalidates: (cause) => cause instanceof ApiError && cause.unauthorized,

    fetchSummaries: (abort) => api.requests(code, invite, abort),

    openStream(handlers) {
      const source = new EventSource(eventsUrl(code, invite))
      source.onopen = () => handlers.open()
      source.onerror = () => handlers.failed()
      source.addEventListener('request', (event) => {
        try {
          handlers.request(JSON.parse((event as MessageEvent).data) as RequestSummary)
        } catch {
          // A summary we cannot read is not worth a broken page: the next
          // refetch carries the same capture from SQLite.
        }
      })
      source.addEventListener('refresh', () => handlers.refresh())
      return { close: () => source.close() }
    },

    async stillAuthorized() {
      try {
        await api.binAccess(code, invite)
        return true
      } catch (cause) {
        return !(cause instanceof ApiError && cause.unauthorized)
      }
    },

    visible: () => document.visibilityState === 'visible',

    onVisibilityChange(listener) {
      document.addEventListener('visibilitychange', listener)
      window.addEventListener('online', listener)
      return () => {
        document.removeEventListener('visibilitychange', listener)
        window.removeEventListener('online', listener)
      }
    },
  }
}
