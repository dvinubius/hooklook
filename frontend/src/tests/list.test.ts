import { describe, expect, it } from 'vitest'
import {
  compareIds,
  filterSummaries,
  filtersActive,
  matchesFilters,
  presentMethods,
  sortSummaries,
} from '../lib/list'
import { emptyFilters, type RequestSummary, type TextFilter } from '../types'

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

function matching(text: string): TextFilter {
  return { operator: 'matches', text }
}

describe('matchesFilters', () => {
  const item = summary('1', { method: 'POST', path: '/Orders/42', rawQuery: 'retry=1&mode=live' })

  it('matches the method exactly, ignoring case', () => {
    expect(matchesFilters(item, { ...emptyFilters(), method: 'post' })).toBe(true)
    expect(matchesFilters(item, { ...emptyFilters(), method: 'GET' })).toBe(false)
  })

  it('matches path and query as case-insensitive substrings', () => {
    expect(matchesFilters(item, { ...emptyFilters(), path: matching('orders') })).toBe(true)
    expect(matchesFilters(item, { ...emptyFilters(), path: matching(' /ORDERS/42 ') })).toBe(true)
    expect(matchesFilters(item, { ...emptyFilters(), path: matching('invoices') })).toBe(false)
    expect(matchesFilters(item, { ...emptyFilters(), query: matching('mode=live') })).toBe(true)
    expect(matchesFilters(item, { ...emptyFilters(), query: matching('mode=test') })).toBe(false)
  })

  it('asks only whether path or query has anything in it', () => {
    const bare = summary('2', { path: '', rawQuery: '' })
    const empty = { operator: 'empty', text: 'ignored' } as const
    const notEmpty = { operator: 'notEmpty', text: '' } as const
    expect(matchesFilters(bare, { ...emptyFilters(), path: empty, query: empty })).toBe(true)
    expect(matchesFilters(bare, { ...emptyFilters(), path: notEmpty })).toBe(false)
    expect(matchesFilters(item, { ...emptyFilters(), path: empty })).toBe(false)
    expect(matchesFilters(item, { ...emptyFilters(), path: notEmpty, query: notEmpty })).toBe(true)
  })

  it('counts a lone slash as an empty path', () => {
    const slash = summary('3', { path: '/' })
    expect(matchesFilters(slash, { ...emptyFilters(), path: { operator: 'empty', text: '' } })).toBe(true)
    expect(matchesFilters(slash, { ...emptyFilters(), path: { operator: 'notEmpty', text: '' } })).toBe(false)
  })

  it('requires every stated filter at once', () => {
    const path = matching('orders')
    const query = matching('retry')
    expect(matchesFilters(item, { method: 'POST', path, query })).toBe(true)
    expect(matchesFilters(item, { method: 'GET', path, query })).toBe(false)
  })

  it('treats whitespace as no filter at all, but an emptiness test as one', () => {
    expect(filtersActive(emptyFilters())).toBe(false)
    expect(filtersActive({ ...emptyFilters(), path: matching('   ') })).toBe(false)
    expect(filtersActive({ ...emptyFilters(), path: matching('/x') })).toBe(true)
    expect(filtersActive({ ...emptyFilters(), query: { operator: 'empty', text: '' } })).toBe(true)
    expect(matchesFilters(item, { ...emptyFilters(), path: matching('   ') })).toBe(true)
  })
})

describe('filterSummaries and presentMethods', () => {
  it('keeps only matching rows and offers the methods actually seen', () => {
    const items = [
      summary('1', { method: 'GET' }),
      summary('2', { method: 'POST' }),
      summary('3', { method: 'get' }),
    ]
    expect(filterSummaries(items, { ...emptyFilters(), method: 'GET' }).map((i) => i.id)).toEqual(['1', '3'])
    expect(presentMethods(items)).toEqual(['GET', 'POST'])
  })
})
