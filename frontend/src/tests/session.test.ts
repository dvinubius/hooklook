import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api'
import {
  createSession,
  recentlyRecovered,
  recoveryWindowMs,
  type Recovery,
  type SessionEnvironment,
} from '../lib/session'
import type { BinAccess } from '../types'

const origin = 'https://hooklook.example'

const ownerAccess: BinAccess = {
  bin: {
    code: 'brave-otter-19472841',
    createdAt: '2026-09-18T08:00:00Z',
    expiresAt: '2026-09-25T08:00:00Z',
    totalBodyBytes: 0,
  },
  owner: true,
  sharingEnabled: false,
  inviteId: 'invitation-identifier',
  capacity: {
    requestCount: 0,
    requestLimit: 500,
    bodyBytesUsed: 0,
    bodyBytesLimit: 100_000_000,
    requestsFull: false,
    bodyBytesFull: false,
    full: false,
  },
  storeCapacity: {
    maxBytes: 5_000_000_000,
    databaseBytes: 32_768,
    reusableBytes: 0,
    availableBytes: 4_999_967_232,
    full: false,
  },
}

interface Harness {
  env: SessionEnvironment
  navigations: string[]
  marks: Array<Recovery | null>
  calls: Array<{ code: string; invite: string }>
}

function harness(
  answer: (attempt: number) => Promise<BinAccess>,
  options: {
    href?: string
    mark?: Recovery | null
    now?: number
    startup?: SessionEnvironment['startupState']
  } = {},
): Harness {
  const navigations: string[] = []
  const marks: Array<Recovery | null> = []
  const calls: Array<{ code: string; invite: string }> = []
  let mark = options.mark ?? null
  let attempt = 0

  const env: SessionEnvironment = {
    href: options.href ?? `${origin}/bins/brave-otter-19472841`,
    origin,
    startupState: options.startup,
    now: () => options.now ?? 1_000_000,
    navigate: (to) => void navigations.push(to),
    readRecovery: () => mark,
    writeRecovery: (next) => {
      mark = next
      marks.push(next)
    },
    fetchAccess: (code, invite) => {
      calls.push({ code, invite })
      attempt += 1
      return answer(attempt)
    },
  }
  return { env, navigations, marks, calls }
}

describe('recentlyRecovered', () => {
  it('counts a recovery inside the window, in either time direction', () => {
    expect(recentlyRecovered(null, 1000)).toBe(false)
    expect(recentlyRecovered({ from: 'code', at: 1000 }, 1000 + recoveryWindowMs / 2)).toBe(true)
    expect(recentlyRecovered({ from: 'code', at: 1000 }, 1000 + recoveryWindowMs)).toBe(false)
    // A clock that moved backwards must stop the loop, not license another one.
    expect(recentlyRecovered({ from: 'code', at: 1000 }, 1000 - recoveryWindowMs / 2)).toBe(true)
  })
})

describe('bin session', () => {
  it('holds private state only after an authorized metadata response', async () => {
    const { env, calls, marks } = harness(async () => ownerAccess)
    const session = createSession(env)

    expect(session.state.value).toBe('loading')
    expect(session.access.value).toBeNull()

    await session.start()

    expect(calls).toEqual([{ code: 'brave-otter-19472841', invite: '' }])
    expect(session.state.value).toBe('ready')
    expect(session.access.value?.owner).toBe(true)
    // A load that worked clears the loop guard for the next real failure.
    expect(marks).toEqual([null])
  })

  it('sends a guest invitation with the metadata call', async () => {
    const { env, calls } = harness(async () => ({ ...ownerAccess, owner: false }), {
      href: `${origin}/bins/brave-otter-19472841/requests/42?invite=abc123`,
    })
    const session = createSession(env)
    await session.start()

    expect(calls).toEqual([{ code: 'brave-otter-19472841', invite: 'abc123' }])
    expect(session.page.requestId).toBe('42')
    expect(session.access.value?.owner).toBe(false)
  })

  it('shows the shared-bin dead end when the bin is not this visitor’s to see', async () => {
    const { env, navigations, marks } = harness(async () => {
      throw new ApiError(404, 'bin not found')
    })
    const session = createSession(env)
    const closed = vi.fn()
    session.onStop(closed)

    await session.start()

    expect(session.state.value).toBe('shared_bin_unavailable')
    expect(session.access.value).toBeNull()
    expect(navigations).toEqual([])
    expect(marks).toEqual([])
    expect(closed).toHaveBeenCalledOnce()
    expect(session.signal.aborted).toBe(true)
  })

  it('treats a revoked invitation the same way', async () => {
    const { env, navigations } = harness(async () => {
      throw new ApiError(403, 'forbidden')
    })
    const session = createSession(env)
    await session.start()

    expect(session.state.value).toBe('shared_bin_unavailable')
    expect(navigations).toEqual([])
  })

  it('shows the expired-bin dead end when the server identifies it', async () => {
    const { env, navigations } = harness(async () => {
      throw new ApiError(404, 'bin not found', 'bin_expired')
    })
    const session = createSession(env)
    await session.start()

    expect(navigations).toEqual([])
    expect(session.state.value).toBe('bin_expired')
  })

  it('keeps a transient failure recoverable instead of redirecting', async () => {
    const { env, navigations } = harness(async (attempt) => {
      if (attempt === 1) throw new ApiError(500, 'internal server error')
      return ownerAccess
    })
    const session = createSession(env)

    await session.start()
    expect(session.state.value).toBe('unavailable')
    expect(session.message.value).toContain('500')
    expect(navigations).toEqual([])

    await session.retry()
    expect(session.state.value).toBe('ready')
    expect(session.access.value?.bin.code).toBe('brave-otter-19472841')
  })

  it('reports an unreachable server as recoverable', async () => {
    const { env, navigations } = harness(async () => {
      throw new TypeError('Failed to fetch')
    })
    const session = createSession(env)
    await session.start()

    expect(session.state.value).toBe('unavailable')
    expect(session.message.value).toMatch(/could not be reached/)
    expect(navigations).toEqual([])
  })

  it('recognizes a store with no room for a bin, and asks it nothing', async () => {
    const { env, navigations, calls } = harness(async () => ownerAccess, {
      href: `${origin}/`,
      startup: 'store_full',
    })
    const session = createSession(env)
    await session.start()

    // Go marked the document; there is no bin, no cookie and nothing to
    // authorize, so the session neither calls the API nor redirects.
    expect(session.state.value).toBe('store_full')
    expect(calls).toEqual([])
    expect(navigations).toEqual([])
  })

  it.each(['bin_expired', 'shared_bin_unavailable'] as const)(
    'recognizes the marked %s page, and asks it nothing',
    async (startup) => {
      const { env, navigations, calls } = harness(async () => ownerAccess, {
        href: `${origin}/bins/old-bin?invite=old-invitation`,
        startup,
      })
      const session = createSession(env)
      await session.start()

      expect(session.state.value).toBe(startup)
      expect(calls).toEqual([])
      expect(navigations).toEqual([])
    },
  )

  it('sends a page without a bin code home', async () => {
    const { env, navigations, calls } = harness(async () => ownerAccess, {
      href: `${origin}/somewhere-else`,
    })
    const session = createSession(env)
    await session.start()

    expect(calls).toEqual([])
    expect(navigations).toEqual(['/'])
  })

  it('ignores an answer that arrives after the session stopped', async () => {
    let settle: (access: BinAccess) => void = () => {}
    const { env } = harness(() => new Promise<BinAccess>((resolve) => (settle = resolve)))
    const session = createSession(env)

    const started = session.start()
    session.stop()
    settle(ownerAccess)
    await started

    expect(session.state.value).toBe('loading')
    expect(session.access.value).toBeNull()
  })

  it('invalidates on demand when the server later denies access', async () => {
    const { env, navigations } = harness(async () => ownerAccess)
    const session = createSession(env)
    await session.start()

    session.invalidate()

    expect(session.state.value).toBe('shared_bin_unavailable')
    expect(navigations).toEqual([])
  })
})
