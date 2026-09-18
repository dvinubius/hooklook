import { describe, expect, it } from 'vitest'
import {
  compareIds,
  filterSummaries,
  filtersActive,
  matchesFilters,
  presentMethods,
  sortSummaries,
} from '../lib/list'
import { emptyFilters, type RequestSummary } from '../types'

function summary(id: string, overrides: Partial<RequestSummary> = {}): RequestSummary {
  return {
    id,
    method: 'POST',
    path: '/orders',
    rawQuery: '',
    receivedAt: '2026-09-18T08:00:00Z',
    contentType: 'application/json',
    bodySizeKiB: 1,
    headerCount: 6,
    ...overrides,
  }
}

describe('compareIds', () => {
  it('compares row ids as numbers, so 9 comes before 10', () => {
    expect(compareIds('9', '10')).toBeLessThan(0)
    expect(compareIds('10', '9')).toBeGreaterThan(0)
    expect(compareIds('7', '7')).toBe(0)
  })
})

describe('sortSummaries', () => {
  it('orders by receipt time and breaks ties by id', () => {
    // Receipt time is truncated to the second, so ties are the common case.
    const items = [
      summary('10', { receivedAt: '2026-09-18T08:00:01Z' }),
      summary('9', { receivedAt: '2026-09-18T08:00:01Z' }),
      summary('3', { receivedAt: '2026-09-18T08:00:00Z' }),
    ]
    expect(sortSummaries(items, 'oldest').map((item) => item.id)).toEqual(['3', '9', '10'])
    expect(sortSummaries(items, 'newest').map((item) => item.id)).toEqual(['10', '9', '3'])
  })

  it('leaves the array it was given alone', () => {
    const items = [summary('2'), summary('1')]
    sortSummaries(items, 'newest')
    expect(items.map((item) => item.id)).toEqual(['2', '1'])
  })

  it('falls back to id order when a time cannot be read', () => {
    const items = [summary('2', { receivedAt: 'not a time' }), summary('1', { receivedAt: 'nor this' })]
    expect(sortSummaries(items, 'oldest').map((item) => item.id)).toEqual(['1', '2'])
  })
})

describe('matchesFilters', () => {
  const item = summary('1', { method: 'POST', path: '/Orders/42', rawQuery: 'retry=1&mode=live' })

  it('matches the method exactly, ignoring case', () => {
    expect(matchesFilters(item, { ...emptyFilters, method: 'post' })).toBe(true)
    expect(matchesFilters(item, { ...emptyFilters, method: 'GET' })).toBe(false)
  })

  it('matches path and query as case-insensitive substrings', () => {
    expect(matchesFilters(item, { ...emptyFilters, path: 'orders' })).toBe(true)
    expect(matchesFilters(item, { ...emptyFilters, path: ' /ORDERS/42 ' })).toBe(true)
    expect(matchesFilters(item, { ...emptyFilters, path: 'invoices' })).toBe(false)
    expect(matchesFilters(item, { ...emptyFilters, query: 'mode=live' })).toBe(true)
    expect(matchesFilters(item, { ...emptyFilters, query: 'mode=test' })).toBe(false)
  })

  it('requires every stated filter at once', () => {
    expect(matchesFilters(item, { method: 'POST', path: 'orders', query: 'retry' })).toBe(true)
    expect(matchesFilters(item, { method: 'GET', path: 'orders', query: 'retry' })).toBe(false)
  })

  it('treats whitespace as no filter at all', () => {
    expect(filtersActive(emptyFilters)).toBe(false)
    expect(filtersActive({ ...emptyFilters, path: '   ' })).toBe(false)
    expect(filtersActive({ ...emptyFilters, path: '/x' })).toBe(true)
    expect(matchesFilters(item, { ...emptyFilters, path: '   ' })).toBe(true)
  })
})

describe('filterSummaries and presentMethods', () => {
  it('keeps only matching rows and offers the methods actually seen', () => {
    const items = [
      summary('1', { method: 'GET' }),
      summary('2', { method: 'POST' }),
      summary('3', { method: 'get' }),
    ]
    expect(filterSummaries(items, { ...emptyFilters, method: 'GET' }).map((i) => i.id)).toEqual(['1', '3'])
    expect(presentMethods(items)).toEqual(['GET', 'POST'])
  })
})
