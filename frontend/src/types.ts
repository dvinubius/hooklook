// Shapes mirror the Go JSON responses in store.go, store_inspection.go and
// requests.go. Times arrive as RFC 3339 strings.

export interface Bin {
  code: string
  createdAt: string
  expiresAt: string
  totalBodyBytes: number
}

/** `GET /api/bins/{code}`. `inviteId` is present for owners only. */
export interface BinAccess {
  bin: Bin
  owner: boolean
  sharingEnabled: boolean
  inviteId?: string
}

/** Body-free list row, and the payload of an SSE `request` event. */
export interface RequestSummary {
  id: string
  method: string
  path: string
  rawQuery: string
  receivedAt: string
  contentType: string
  bodySizeKiB: number
  headerCount: number
}

/** `GET /api/bins/{code}/requests/{id}`. `rawBody` is base64 from Go's []byte. */
export interface RequestDetail extends RequestSummary {
  headers: Record<string, string[]>
  rawBody: string | null
}

export type SortOrder = 'newest' | 'oldest'

/** How a text filter tests its field: a case-insensitive substring, or only
 *  whether the field has anything in it. `text` counts for `matches` only. */
export type TextOperator = 'matches' | 'empty' | 'notEmpty'

export interface TextFilter {
  operator: TextOperator
  text: string
}

export interface RequestFilters {
  method: string
  path: TextFilter
  query: TextFilter
}

/** A fresh set each call: the text filters are objects the list edits in place. */
export function emptyFilters(): RequestFilters {
  return {
    method: '',
    path: { operator: 'matches', text: '' },
    query: { operator: 'matches', text: '' },
  }
}

export type StreamState = 'connecting' | 'live' | 'reconnecting' | 'closed'
