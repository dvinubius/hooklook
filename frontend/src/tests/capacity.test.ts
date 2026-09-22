import { describe, expect, it } from 'vitest'
import {
  capacityAlarming,
  requestsDetail,
  requestsPercent,
  storageDetail,
  storagePercent,
} from '../lib/capacity'
import type { BinCapacity } from '../types'

function capacity(fields: Partial<BinCapacity> = {}): BinCapacity {
  const base: BinCapacity = {
    requestCount: 0,
    requestLimit: 500,
    bodyBytesUsed: 0,
    bodyBytesLimit: 100_000_000,
    requestsFull: false,
    bodyBytesFull: false,
    full: false,
  }
  const merged = { ...base, ...fields }
  return { ...merged, full: merged.full || merged.requestsFull || merged.bodyBytesFull }
}

describe('storagePercent and requestsPercent', () => {
  it('read their own limit, not each other\'s', () => {
    // A tenth of the request slots, four fifths of the bytes: a bin can be
    // nearly out of one while the other has room to spare.
    const used = capacity({ requestCount: 50, bodyBytesUsed: 80_000_000 })
    expect(storagePercent(used)).toBe(80)
    expect(requestsPercent(used)).toBe(10)
  })

  it('are zero for an untouched bin', () => {
    expect(storagePercent(capacity())).toBe(0)
    expect(requestsPercent(capacity())).toBe(0)
  })

  it('floor, so they never claim room that is already spent', () => {
    expect(requestsPercent(capacity({ requestCount: 249 }))).toBe(49)
  })

  it('stop at 99 while the bin still takes captures', () => {
    expect(requestsPercent(capacity({ requestCount: 499 }))).toBe(99)
    expect(storagePercent(capacity({ bodyBytesUsed: 99_999_999 }))).toBe(99)
  })

  it('read 100 only when the server says that limit is reached', () => {
    const slotsFull = capacity({ requestCount: 500, requestsFull: true })
    expect(requestsPercent(slotsFull)).toBe(100)
    expect(storagePercent(slotsFull)).toBe(0)

    const bytesFull = capacity({ bodyBytesUsed: 100_000_000, bodyBytesFull: true })
    expect(storagePercent(bytesFull)).toBe(100)
  })

  it('treat an absent limit as no limit rather than dividing by zero', () => {
    expect(requestsPercent(capacity({ requestCount: 3, requestLimit: 0 }))).toBe(0)
  })
})

describe('capacityAlarming', () => {
  it('turns over past the warning mark, not at it', () => {
    expect(capacityAlarming(90)).toBe(false)
    expect(capacityAlarming(91)).toBe(true)
    expect(capacityAlarming(100)).toBe(true)
  })
})

describe('storageDetail and requestsDetail', () => {
  it('name the limit each one is about', () => {
    const used = capacity({ requestCount: 50, bodyBytesUsed: 2_500_000 })
    expect(storageDetail(used)).toBe('2.5 of 100.0 MB of request bodies')
    expect(requestsDetail(used)).toBe('50 of 500 requests')
  })

  it('say when a limit is reached', () => {
    expect(requestsDetail(capacity({ requestCount: 500, requestsFull: true }))).toContain(
      'the limit is reached',
    )
    expect(
      storageDetail(capacity({ bodyBytesUsed: 100_000_000, bodyBytesFull: true })),
    ).toContain('the limit is reached')
  })
})
