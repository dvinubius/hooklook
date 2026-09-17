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

export interface RequestFilters {
  method: string
  path: string
  query: string
}

export const emptyFilters: RequestFilters = { method: '', path: '', query: '' }

export type StreamState = 'connecting' | 'live' | 'reconnecting' | 'closed'
