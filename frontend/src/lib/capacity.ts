/** How full a bin is, limit by limit.
 *
 * A bin has two independent limits — a raw-body byte budget and a request
 * count — and a capture is refused as soon as *either* is reached. They run
 * down at their own rates, so each is read on its own rather than folded into
 * one number: a bin can be out of bytes with four hundred slots to spare.
 *
 * A percentage never rounds up to 100 on its own: only the server's own flag
 * says a limit is reached, and the reader should not see "100%" beside a bin
 * that still takes captures. It floors for the same reason. */

import type { BinCapacity } from '../types'

/** Above this a reading is a warning rather than room. */
export const capacityWarningPercent = 90

/** 0–100, whole percent. 100 only when the server says the limit is reached. */
function usagePercent(used: number, limit: number, reached: boolean): number {
  if (reached) return 100
  if (!Number.isFinite(used) || !Number.isFinite(limit) || limit <= 0) return 0
  return Math.min(99, Math.floor((Math.max(0, used) / limit) * 100))
}

/** The raw-body byte budget. */
export function storagePercent(capacity: BinCapacity): number {
  return usagePercent(capacity.bodyBytesUsed, capacity.bodyBytesLimit, capacity.bodyBytesFull)
}

/** The request-count budget. */
export function requestsPercent(capacity: BinCapacity): number {
  return usagePercent(capacity.requestCount, capacity.requestLimit, capacity.requestsFull)
}

/** True once a reading should be brick rather than green. */
export function capacityAlarming(percent: number): boolean {
  return percent > capacityWarningPercent
}

function megabytes(bytes: number): string {
  return (bytes / 1_000_000).toFixed(1)
}

/** What the storage row's title says. */
export function storageDetail(capacity: BinCapacity): string {
  const counts = `${megabytes(capacity.bodyBytesUsed)} of ${megabytes(capacity.bodyBytesLimit)} MB of request bodies`
  return capacity.bodyBytesFull ? `${counts} — the limit is reached` : counts
}

/** What the requests row's title says. */
export function requestsDetail(capacity: BinCapacity): string {
  const counts = `${capacity.requestCount} of ${capacity.requestLimit} requests`
  return capacity.requestsFull ? `${counts} — the limit is reached` : counts
}
