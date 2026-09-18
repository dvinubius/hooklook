import { describe, expect, it } from 'vitest'
import { ApiError } from '../api'
import { createFeed, type FeedEnvironment, type StreamHandlers } from '../lib/feed'
import type { RequestSummary } from '../types'

function summary(id: string): RequestSummary {
  return {
    id,
    method: 'POST',
    path: `/orders/${id}`,
    rawQuery: '',
    receivedAt: '2026-09-18T08:00:00Z',
    contentType: 'application/json',
    bodySizeKiB: 1,
    headerCount: 5,
  }
}

interface Harness {
  env: FeedEnvironment
  handlers: StreamHandlers
  /** Resolves the fetch that is currently in flight. */
  answer(rows: RequestSummary[]): Promise<void>
  fail(cause: unknown): Promise<void>
  fetches: number
  closed: number
  revocations: number
  authorized: boolean
  foreground: boolean
  fireVisibility(): void
}

function harness(): Harness {
  let resolve: ((rows: RequestSummary[]) => void) | null = null
  let reject: ((cause: unknown) => void) | null = null
  let handlers: StreamHandlers | null = null
  let listener: (() => void) | null = null

  const state = {
    fetches: 0,
    closed: 0,
    revocations: 0,
    authorized: true,
    foreground: true,
  }

  const env: FeedEnvironment = {
    signal: new AbortController().signal,
    invalidates: (cause) => cause instanceof ApiError && cause.unauthorized,
    revoked: () => void (state.revocations += 1),
    fetchSummaries: () => {
      state.fetches += 1
      return new Promise<RequestSummary[]>((ok, no) => {
        resolve = ok
        reject = no
      })
    },
    openStream: (given) => {
      handlers = given
      return { close: () => void (state.closed += 1) }
    },
    stillAuthorized: () => Promise.resolve(state.authorized),
    visible: () => state.foreground,
    onVisibilityChange: (given) => {
      listener = given
      return () => {
        listener = null
      }
    },
  }

  return {
    env,
    get handlers() {
      if (!handlers) throw new Error('the stream was never opened')
      return handlers
    },
    answer: async (rows) => {
      resolve?.(rows)
      await Promise.resolve()
      await Promise.resolve()
    },
    fail: async (cause) => {
      reject?.(cause)
      await Promise.resolve()
      await Promise.resolve()
      await Promise.resolve()
    },
    fireVisibility: () => listener?.(),
    get fetches() {
      return state.fetches
    },
    get closed() {
      return state.closed
    },
    get revocations() {
      return state.revocations
    },
    get authorized() {
      return state.authorized
    },
    set authorized(value: boolean) {
      state.authorized = value
    },
    get foreground() {
      return state.foreground
    },
    set foreground(value: boolean) {
      state.foreground = value
    },
  }
}

describe('createFeed', () => {
  it('fetches the persisted list as soon as it starts', async () => {
    const test = harness()
    const feed = createFeed(test.env)
    feed.start()
    expect(feed.loading.value).toBe(true)
    expect(feed.loaded.value).toBe(false)

    await test.answer([summary('1'), summary('2')])
    expect(feed.summaries.value.map((row) => row.id)).toEqual(['1', '2'])
    expect(feed.loading.value).toBe(false)
    expect(feed.loaded.value).toBe(true)
    expect(test.fetches).toBe(1)
  })

  it('keeps an event that raced the fetch it was issued alongside', async () => {
    const test = harness()
    const feed = createFeed(test.env)
    feed.start()

    // The capture happens while the initial fetch is still in flight, so the
    // snapshot that comes back cannot contain it.
    test.handlers.request(summary('3'))
    await test.answer([summary('1'), summary('2')])

    expect(feed.summaries.value.map((row) => row.id)).toEqual(['1', '2', '3'])
  })

  it('reconciles by id rather than appending, so a repeat cannot duplicate', async () => {
    const test = harness()
    const feed = createFeed(test.env)
    feed.start()
    await test.answer([summary('1')])

    test.handlers.request(summary('2'))
    test.handlers.request(summary('2'))
    expect(feed.summaries.value.map((row) => row.id)).toEqual(['1', '2'])

    // And the next snapshot, which now contains it, does not double it either.
    test.handlers.refresh()
    await test.answer([summary('1'), summary('2')])
    expect(feed.summaries.value.map((row) => row.id)).toEqual(['1', '2'])
  })

  it('drops rows a refetch no longer reports, which is how a deletion lands', async () => {
    const test = harness()
    const feed = createFeed(test.env)
    feed.start()
    await test.answer([summary('1'), summary('2')])

    test.handlers.refresh()
    await test.answer([summary('2')])
    expect(feed.summaries.value.map((row) => row.id)).toEqual(['2'])
  })

  it('refetches after a reconnect, because the stream replays nothing', async () => {
    const test = harness()
    const feed = createFeed(test.env)
    feed.start()
    test.handlers.open()
    await test.answer([summary('1')])
    expect(feed.stream.value).toBe('live')
    expect(test.fetches).toBe(1)

    test.handlers.failed()
    expect(feed.stream.value).toBe('reconnecting')
    await Promise.resolve()

    test.handlers.open()
    expect(feed.stream.value).toBe('live')
    expect(test.fetches).toBe(2)
    await test.answer([summary('1'), summary('2')])
    expect(feed.summaries.value.map((row) => row.id)).toEqual(['1', '2'])
  })

  it('does not refetch on the first connection, which already has one running', async () => {
    const test = harness()
    const feed = createFeed(test.env)
    feed.start()
    test.handlers.open()
    expect(test.fetches).toBe(1)
    await test.answer([])
  })

  it('leaves the page when a stream failure turns out to be revoked access', async () => {
    const test = harness()
    const feed = createFeed(test.env)
    feed.start()
    await test.answer([summary('1')])

    test.authorized = false
    test.handlers.failed()
    await Promise.resolve()
    await Promise.resolve()

    expect(test.revocations).toBe(1)
    expect(test.closed).toBe(1)
    expect(feed.stream.value).toBe('closed')
  })

  it('asks once per disconnection, not once per retry', async () => {
    const test = harness()
    const feed = createFeed(test.env)
    feed.start()
    await test.answer([])

    let asked = 0
    const env = test.env as { stillAuthorized: () => Promise<boolean> }
    env.stillAuthorized = () => {
      asked += 1
      return Promise.resolve(true)
    }
    test.handlers.failed()
    test.handlers.failed()
    test.handlers.failed()
    await Promise.resolve()
    expect(asked).toBe(1)
    expect(feed.stream.value).toBe('reconnecting')
  })

  it('leaves the page when the list itself is refused', async () => {
    const test = harness()
    const feed = createFeed(test.env)
    feed.start()
    await test.fail(new ApiError(404, 'bin not found'))

    expect(test.revocations).toBe(1)
    expect(feed.stream.value).toBe('closed')
  })

  it('offers a recoverable error instead of leaving on a server failure', async () => {
    const test = harness()
    const feed = createFeed(test.env)
    feed.start()
    await test.fail(new ApiError(500, 'internal server error'))

    expect(test.revocations).toBe(0)
    expect(feed.error.value).not.toBe('')
    expect(feed.loading.value).toBe(false)

    const retried = feed.refetch()
    await test.answer([summary('1')])
    await retried
    expect(feed.error.value).toBe('')
    expect(feed.summaries.value.map((row) => row.id)).toEqual(['1'])
  })

  it('refetches when a page whose stream is down returns to the foreground', async () => {
    const test = harness()
    const feed = createFeed(test.env)
    feed.start()
    test.handlers.open()
    await test.answer([])
    expect(test.fetches).toBe(1)

    // Live and visible: nothing was missed.
    test.fireVisibility()
    expect(test.fetches).toBe(1)

    test.handlers.failed()
    await Promise.resolve()
    test.foreground = false
    test.fireVisibility()
    expect(test.fetches).toBe(1)

    test.foreground = true
    test.fireVisibility()
    expect(test.fetches).toBe(2)
  })

  it('forgets a row locally so a deleted request does not linger', async () => {
    const test = harness()
    const feed = createFeed(test.env)
    feed.start()
    await test.answer([summary('1'), summary('2')])

    feed.forget('1')
    expect(feed.summaries.value.map((row) => row.id)).toEqual(['2'])
  })

  it('stops everything once, and ignores what arrives afterwards', async () => {
    const test = harness()
    const feed = createFeed(test.env)
    feed.start()
    await test.answer([summary('1')])

    feed.stop()
    feed.stop()
    expect(test.closed).toBe(1)
    expect(feed.stream.value).toBe('closed')

    test.handlers.request(summary('2'))
    test.handlers.refresh()
    expect(feed.summaries.value.map((row) => row.id)).toEqual(['1'])
    expect(test.fetches).toBe(1)
  })
})
