/** Ordering and filtering of the request list.
 *
 * The server sends every summary it has, in ascending id order, and filters
 * nothing. Every arrangement the reader chooses therefore happens here, over
 * whatever the feed currently holds — which is why these are plain functions
 * on arrays rather than another request to the server. */

import type { RequestFilters, RequestSummary, SortOrder } from '../types'

/** Request ids are SQLite row ids sent as strings, so they are compared as
 *  numbers: a string comparison would put 10 before 9. */
export function compareIds(left: string, right: string): number {
  const a = Number(left)
  const b = Number(right)
  if (Number.isFinite(a) && Number.isFinite(b) && a !== b) return a < b ? -1 : 1
  return left < right ? -1 : left > right ? 1 : 0
}

/** Receipt time is truncated to the second, so captures tie often. The id is
 *  the deterministic tie-breaker, which also keeps the order stable as live
 *  events arrive. An unreadable time sorts by id alone. */
function compareSummaries(left: RequestSummary, right: RequestSummary): number {
  const at = Date.parse(left.receivedAt)
  const bt = Date.parse(right.receivedAt)
  if (Number.isFinite(at) && Number.isFinite(bt) && at !== bt) return at < bt ? -1 : 1
  return compareIds(left.id, right.id)
}

export function sortSummaries(
  items: readonly RequestSummary[],
  order: SortOrder,
): RequestSummary[] {
  const sorted = [...items].sort(compareSummaries)
  return order === 'newest' ? sorted.reverse() : sorted
}

function contains(haystack: string, needle: string): boolean {
  return haystack.toLowerCase().includes(needle.trim().toLowerCase())
}

/** Method matches exactly — it is chosen from the methods actually present —
 *  while path and query are substrings, which is how a reader looks for one
 *  delivery among many. */
export function matchesFilters(item: RequestSummary, filters: RequestFilters): boolean {
  if (filters.method !== '' && item.method.toUpperCase() !== filters.method.toUpperCase()) {
    return false
  }
  if (filters.path.trim() !== '' && !contains(item.path, filters.path)) return false
  if (filters.query.trim() !== '' && !contains(item.rawQuery, filters.query)) return false
  return true
}

export function filterSummaries(
  items: readonly RequestSummary[],
  filters: RequestFilters,
): RequestSummary[] {
  return items.filter((item) => matchesFilters(item, filters))
}

export function filtersActive(filters: RequestFilters): boolean {
  return filters.method !== '' || filters.path.trim() !== '' || filters.query.trim() !== ''
}

/** The methods offered in the filter are the ones this bin has really seen. */
export function presentMethods(items: readonly RequestSummary[]): string[] {
  return [...new Set(items.map((item) => item.method.toUpperCase()))].sort()
}
