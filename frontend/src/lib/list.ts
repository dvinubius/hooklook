/** Ordering and filtering of the request list.
 *
 * The server sends every summary it has, in ascending id order, and filters
 * nothing. Every arrangement the reader chooses therefore happens here, over
 * whatever the feed currently holds — which is why these are plain functions
 * on arrays rather than another request to the server. */

import type { RequestFilters, RequestSummary, SortOrder, TextFilter } from '../types'

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

/** A path of "/" is the capture URL itself with a trailing slash, so it
 *  counts as empty, like no path at all. */
function blankPath(path: string): string {
  return path === '/' ? '' : path
}

function textFilterOn(filter: TextFilter): boolean {
  return filter.operator !== 'matches' || filter.text.trim() !== ''
}

/** `matches` is a case-insensitive substring, which is how a reader looks for
 *  one delivery among many; the other two only ask whether there is anything
 *  there. A `matches` with no text lets everything through. */
function passes(value: string, filter: TextFilter): boolean {
  if (filter.operator === 'empty') return value === ''
  if (filter.operator === 'notEmpty') return value !== ''
  const needle = filter.text.trim().toLowerCase()
  return needle === '' || value.toLowerCase().includes(needle)
}

/** Method matches exactly — it is chosen from the methods actually present. */
export function matchesFilters(item: RequestSummary, filters: RequestFilters): boolean {
  if (filters.method !== '' && item.method.toUpperCase() !== filters.method.toUpperCase()) {
    return false
  }
  return passes(blankPath(item.path), filters.path) && passes(item.rawQuery, filters.query)
}

export function filterSummaries(
  items: readonly RequestSummary[],
  filters: RequestFilters,
): RequestSummary[] {
  return items.filter((item) => matchesFilters(item, filters))
}

export function filtersActive(filters: RequestFilters): boolean {
  return filters.method !== '' || textFilterOn(filters.path) || textFilterOn(filters.query)
}

/** The methods offered in the filter are the ones this bin has really seen. */
export function presentMethods(items: readonly RequestSummary[]): string[] {
  return [...new Set(items.map((item) => item.method.toUpperCase()))].sort()
}
