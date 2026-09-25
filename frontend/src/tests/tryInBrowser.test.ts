import { describe, expect, it } from 'vitest'
import { claimTryInBrowser, shownCount, type CookieJar } from '../lib/tryInBrowser'

/** Behaves like document.cookie for one name: a write replaces that cookie
 *  and leaves the others alone. */
function jar(initial = ''): CookieJar & { cookies: string } {
  const values = new Map<string, string>()
  for (const part of initial.split(';')) {
    const [name, value] = part.trim().split('=')
    if (name) values.set(name, value ?? '')
  }
  return {
    get cookies() {
      return [...values].map(([name, value]) => `${name}=${value}`).join('; ')
    },
    read() {
      return this.cookies
    },
    write(cookie) {
      const [name, value] = cookie.split(';')[0].split('=')
      values.set(name, value)
    },
  }
}

describe('Test in the browser', () => {
  it('is offered three times in a browser, then never again', () => {
    const cookies = jar('hooklook_owner=secret')
    expect([1, 2, 3, 4, 5].map(() => claimTryInBrowser(cookies))).toEqual([
      true, true, true, false, false,
    ])
    expect(shownCount(cookies.cookies)).toBe(3)
    expect(cookies.cookies).toContain('hooklook_owner=secret')
  })

  it('writes a long-lived cookie for the whole site', () => {
    let written = ''
    claimTryInBrowser({ read: () => '', write: (cookie) => { written = cookie } })
    expect(written).toMatch(/^hooklook_try_shown=1; Path=\/; Max-Age=\d+; SameSite=Lax/)
  })

  it('treats a missing or malformed count as never shown', () => {
    expect(shownCount('')).toBe(0)
    expect(shownCount('hooklook_try_shown=abc')).toBe(0)
    expect(shownCount('other=2; hooklook_try_shown=2')).toBe(2)
  })

  it('stays hidden when cookies cannot be kept', () => {
    // A browser that drops the write would otherwise show it on every visit.
    expect(claimTryInBrowser({ read: () => '', write: () => {} })).toBe(false)
    expect(claimTryInBrowser({ read: () => { throw new Error('blocked') }, write: () => {} })).toBe(false)
  })
})
