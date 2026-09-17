/** Bin code and optional invitation identifier, read from the current URL.
 *
 * The invitation stays in memory and on same-origin request URLs only: it is
 * never written to storage, logged, or handed to anything third-party. The
 * owner cookie is HttpOnly and is never read here. */

export interface PageLocation {
  /** Bin code from `/bins/{code}` — empty when the path is not a bin page. */
  code: string
  /** Selected request id from `/bins/{code}/requests/{id}`, or null. */
  requestId: string | null
  /** `?invite=...`, or empty when absent. */
  invite: string
}

const binPath = /^\/bins\/([^/]+)(?:\/requests\/([^/]+))?\/?$/

export function parseLocation(url: string, origin: string): PageLocation {
  const parsed = new URL(url, origin)
  const match = binPath.exec(parsed.pathname)
  if (!match) return { code: '', requestId: null, invite: '' }
  return {
    code: decodeURIComponent(match[1]!),
    requestId: match[2] ? decodeURIComponent(match[2]) : null,
    invite: parsed.searchParams.get('invite') ?? '',
  }
}

/** Path for a bin page or a selected request, preserving the invitation. */
export function pagePath(code: string, requestId: string | null, invite: string): string {
  const base = requestId
    ? `/bins/${encodeURIComponent(code)}/requests/${encodeURIComponent(requestId)}`
    : `/bins/${encodeURIComponent(code)}`
  return invite ? `${base}?invite=${encodeURIComponent(invite)}` : base
}

/** The public capture URL for a bin, on the origin the browser is already on. */
export function captureUrl(code: string, origin: string): string {
  return `${origin}/b/${encodeURIComponent(code)}`
}

/** The shareable inspection link for a guest. Never includes the owner cookie. */
export function inviteUrl(code: string, inviteId: string, origin: string): string {
  return `${origin}${pagePath(code, null, inviteId)}`
}
