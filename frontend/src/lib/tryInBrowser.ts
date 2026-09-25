/** The empty bin's "Test in the browser" button is a first-visit nudge: it is
 *  shown three times in this browser, across bins, and then never again. The
 *  count lives in a cookie of its own — the owner cookie is HttpOnly, and this
 *  one carries nothing but the number. */

export const tryInBrowserCookie = 'hooklook_try_shown'
export const tryInBrowserLimit = 3

/** A year is "for good" here; the nudge has no reason to come back sooner. */
const maxAgeSeconds = 365 * 24 * 60 * 60

export interface CookieJar {
  read(): string
  write(cookie: string): void
}

const documentJar: CookieJar = {
  read: () => document.cookie,
  write: (cookie) => {
    document.cookie = cookie
  },
}

export function shownCount(cookies: string): number {
  for (const part of cookies.split(';')) {
    const [name, value] = part.trim().split('=')
    if (name !== tryInBrowserCookie) continue
    const count = Number.parseInt(value ?? '', 10)
    return Number.isFinite(count) && count > 0 ? count : 0
  }
  return 0
}

/** Decides whether this appearance of the empty bin shows the button, and
 *  counts it when it does. A blocked cookie API hides the button rather than
 *  showing it forever. */
export function claimTryInBrowser(jar: CookieJar = documentJar): boolean {
  try {
    const count = shownCount(jar.read())
    if (count >= tryInBrowserLimit) return false
    const secure = typeof location !== 'undefined' && location.protocol === 'https:' ? '; Secure' : ''
    jar.write(`${tryInBrowserCookie}=${count + 1}; Path=/; Max-Age=${maxAgeSeconds}; SameSite=Lax${secure}`)
    return shownCount(jar.read()) === count + 1
  } catch {
    return false
  }
}
