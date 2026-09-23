/** The page session: which bin this page is about, and whether this browser is
 *  still allowed to see it.
 *
 * Go authorized the document before the browser ran a line of this, but the
 * page must not render private data on that basis alone: a bin expires, an
 * invitation is revoked, a restored back/forward page outlives both. So the
 * application asks `GET /api/bins/{code}` before anything private loads, and
 * treats that answer as the only source of role and sharing state.
 *
 * Two failures, two behaviors. `403`/`404` stops everything in flight and
 * shows the marked unavailable-bin page, where creating a replacement is an
 * explicit choice. A network or server failure means we do not know yet: that
 * is recoverable and retried on request.
 *
 * The invitation stays in this module's memory and on same-origin request URLs
 * only. Nothing here writes it to storage, logs it, or hands it to anything
 * third-party. The owner cookie is `HttpOnly` and is never read at all. */

import { ref, type Ref } from 'vue'
import { api, ApiError } from '../api'
import { parseLocation, type PageLocation } from './location'
import type { BinAccess } from '../types'

/** Marked documents have no bin session to authorize. */
export type StartupState = 'store_full' | 'bin_expired' | 'shared_bin_unavailable'
export type SessionState = 'loading' | 'ready' | 'unavailable' | 'leaving' | StartupState

/** Whether a failure means "not for you" rather than "not right now". */
function invalidates(cause: unknown): boolean {
  return cause instanceof ApiError && cause.unauthorized
}

/** A note that this browser has just been sent through `/` to recover. The bin
 *  code it carries is an address, never a credential — no invitation, no
 *  cookie, nothing derived from either. */
export interface Recovery {
  from: string
  at: number
}

/** How long one recovery keeps counting. Long enough to cover a redirect
 *  through `/` and a fresh page load, short enough that a real failure an hour
 *  later may still recover. */
export const recoveryWindowMs = 20_000

export function recentlyRecovered(mark: Recovery | null, now: number): boolean {
  // Absolute difference: a clock that moved backwards should stop the loop,
  // not license another one.
  return mark !== null && Math.abs(now - mark.at) < recoveryWindowMs
}

/** Everything the session touches outside itself, so the whole bootstrap can
 *  be exercised without a browser. */
export interface SessionEnvironment {
  href: string
  origin: string
  startupState?: StartupState
  now(): number
  navigate(to: string): void
  readRecovery(): Recovery | null
  writeRecovery(mark: Recovery | null): void
  fetchAccess(code: string, invite: string, signal: AbortSignal): Promise<BinAccess>
}

const recoveryKey = 'hooklook.recovery'

export function browserEnvironment(): SessionEnvironment {
  const markedState = document.documentElement.getAttribute('data-hooklook-startup')
  return {
    href: window.location.href,
    origin: window.location.origin,
    startupState:
      markedState === 'store_full' ||
      markedState === 'bin_expired' ||
      markedState === 'shared_bin_unavailable'
        ? markedState
        : undefined,
    now: () => Date.now(),
    navigate: (to) => window.location.assign(to),

    readRecovery() {
      try {
        const raw = sessionStorage.getItem(recoveryKey)
        if (!raw) return null
        const parsed = JSON.parse(raw) as Partial<Recovery>
        if (typeof parsed.from !== 'string' || typeof parsed.at !== 'number') return null
        return { from: parsed.from, at: parsed.at }
      } catch {
        return null
      }
    },

    writeRecovery(mark) {
      try {
        if (mark === null) sessionStorage.removeItem(recoveryKey)
        else sessionStorage.setItem(recoveryKey, JSON.stringify(mark))
      } catch {
        // A browser that refuses storage loses the loop guard, not the
        // application: it still gets one redirect per failed session.
      }
    },

    fetchAccess: (code, invite, signal) => api.binAccess(code, invite, signal),
  }
}

export interface BinSession {
  /** Bin code, optional selected request, and invitation, read from the URL. */
  readonly page: PageLocation
  readonly state: Ref<SessionState>
  readonly access: Ref<BinAccess | null>
  readonly message: Ref<string>
  /** Cancels every dependent request when the session ends. */
  readonly signal: AbortSignal
  start(): Promise<void>
  retry(): Promise<void>
  /** Re-reads role and sharing state after the owner changed them, without
   *  tearing the page down: this is a settings update, not a bootstrap. */
  refreshAccess(): Promise<void>
  /** Call when a live page learns that its shared access is gone. */
  invalidate(): void
  /** Register a stream or subscription to close when the session ends. */
  onStop(teardown: () => void): void
  stop(): void
}

export function createSession(env: SessionEnvironment): BinSession {
  const page = parseLocation(env.href, env.origin)
  const state = ref<SessionState>('loading')
  const access = ref<BinAccess | null>(null)
  const message = ref('')
  const controller = new AbortController()
  const teardowns: Array<() => void> = []

  function stop(): void {
    for (const teardown of teardowns.splice(0)) teardown()
    controller.abort()
  }

  function leave(): void {
    if (recentlyRecovered(env.readRecovery(), env.now())) {
      // We arrived here from a failed session moments ago. Redirecting again
      // would bounce the browser between `/` and a bin page indefinitely.
      stop()
      access.value = null
      state.value = 'unavailable'
      message.value =
        'This bin is not available to you, and the bin you were sent to instead did not load either. Reload to try again.'
      return
    }
    env.writeRecovery({ from: page.code, at: env.now() })
    stop()
    access.value = null
    state.value = 'leaving'
    env.navigate('/')
  }

  function showUnavailable(cause?: unknown): void {
    stop()
    access.value = null
    message.value = ''
    state.value =
      cause instanceof ApiError && cause.code === 'bin_expired'
        ? 'bin_expired'
        : 'shared_bin_unavailable'
  }

  async function load(): Promise<void> {
    if (controller.signal.aborted) return
    state.value = 'loading'
    message.value = ''
    try {
      const authorized = await env.fetchAccess(page.code, page.invite, controller.signal)
      // A response can win the race against its own cancellation. An ended
      // session must not start showing private data because of that.
      if (controller.signal.aborted) return
      access.value = authorized
      env.writeRecovery(null)
      state.value = 'ready'
    } catch (cause) {
      if (controller.signal.aborted) return
      if (invalidates(cause)) {
        showUnavailable(cause)
        return
      }
      access.value = null
      state.value = 'unavailable'
      message.value =
        cause instanceof ApiError
          ? `The server answered ${cause.status}. Your access is fine; the bin could not be read right now.`
          : 'hooklook could not be reached. Check the connection, then try again.'
    }
  }

  async function refreshAccess(): Promise<void> {
    if (controller.signal.aborted) return
    try {
      const authorized = await env.fetchAccess(page.code, page.invite, controller.signal)
      if (controller.signal.aborted) return
      access.value = authorized
    } catch (cause) {
      if (controller.signal.aborted) return
      // An owner who just made a change is not thrown off their own page over
      // a failed re-read; the change either applied or reported its own error.
      if (invalidates(cause)) showUnavailable(cause)
    }
  }

  async function start(): Promise<void> {
    if (env.startupState) {
      state.value = env.startupState
      message.value = ''
      return
    }
    // Go only serves this application on bin pages, so a path without a code
    // means the browser is somewhere it was never handed — send it home.
    if (page.code === '') {
      leave()
      return
    }
    await load()
  }

  return {
    page,
    state,
    access,
    message,
    get signal() {
      return controller.signal
    },
    start,
    retry: load,
    refreshAccess,
    invalidate: showUnavailable,
    onStop: (teardown) => void teardowns.push(teardown),
    stop,
  }
}
