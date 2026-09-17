/** The single place that talks to the Go service.
 *
 * Every call is same-origin so the HttpOnly owner cookie rides along and the
 * server's `Origin` check on mutations passes. Guests pass their invitation on
 * each request; the backend re-authorizes every time. */

import type { BinAccess, RequestDetail, RequestSummary } from './types'

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }

  /** The bin is gone, or this visitor is no longer authorized for it. */
  get unauthorized(): boolean {
    return this.status === 404 || this.status === 403
  }
}

function apiPath(code: string, suffix: string, invite: string): string {
  const path = `/api/bins/${encodeURIComponent(code)}${suffix}`
  return invite ? `${path}?invite=${encodeURIComponent(invite)}` : path
}

export function eventsUrl(code: string, invite: string): string {
  return apiPath(code, '/events', invite)
}

async function failure(response: Response): Promise<ApiError> {
  let body = ''
  try {
    body = (await response.text()).trim()
  } catch {
    body = ''
  }
  return new ApiError(response.status, body || `request failed with ${response.status}`)
}

async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    method: 'GET',
    credentials: 'same-origin',
    headers: { Accept: 'application/json' },
    cache: 'no-store',
    signal,
  })
  if (!response.ok) throw await failure(response)
  return (await response.json()) as T
}

async function mutate(method: string, path: string, body?: unknown): Promise<Response> {
  const response = await fetch(path, {
    method,
    credentials: 'same-origin',
    cache: 'no-store',
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    // The replace endpoint answers 303 to the new bin page. Following it would
    // fetch HTML we do not want; we read the location ourselves instead.
    redirect: 'manual',
  })
  if (!response.ok && response.type !== 'opaqueredirect' && response.status !== 303) {
    throw await failure(response)
  }
  return response
}

export const api = {
  binAccess: (code: string, invite: string, signal?: AbortSignal) =>
    getJSON<BinAccess>(apiPath(code, '', invite), signal),

  requests: (code: string, invite: string, signal?: AbortSignal) =>
    getJSON<RequestSummary[]>(apiPath(code, '/requests', invite), signal),

  requestDetail: (code: string, id: string, invite: string, signal?: AbortSignal) =>
    getJSON<RequestDetail>(apiPath(code, `/requests/${encodeURIComponent(id)}`, invite), signal),

  deleteRequest: (code: string, id: string) =>
    mutate('DELETE', apiPath(code, `/requests/${encodeURIComponent(id)}`, '')).then(() => undefined),

  clearRequests: (code: string) =>
    mutate('DELETE', apiPath(code, '/requests', '')).then(() => undefined),

  setSharing: (code: string, enabled: boolean) =>
    mutate('PUT', apiPath(code, '/sharing', ''), { enabled }).then(() => undefined),

  /** Replaces the bin and sets a new owner cookie. Returns nothing usable: the
   *  caller navigates to `/` so Go resolves whatever the cookie now owns. */
  replaceBin: (code: string) =>
    mutate('POST', apiPath(code, '/replace', '')).then(() => undefined),
}
