import { describe, expect, it } from 'vitest'
import { formatInstant, untilExpiry } from '../lib/format'

const now = Date.parse('2026-09-18T08:00:00Z')

describe('untilExpiry', () => {
  it('names the largest whole unit and nothing finer', () => {
    expect(untilExpiry('2026-09-25T08:00:00Z', now)).toBe('7 days')
    expect(untilExpiry('2026-09-19T20:00:00Z', now)).toBe('1 day')
    expect(untilExpiry('2026-09-19T07:00:00Z', now)).toBe('23 hours')
    expect(untilExpiry('2026-09-18T09:00:00Z', now)).toBe('1 hour')
    expect(untilExpiry('2026-09-18T08:02:30Z', now)).toBe('2 minutes')
  })

  it('stops counting in the last minute', () => {
    expect(untilExpiry('2026-09-18T08:00:59Z', now)).toBe('any second now')
    expect(untilExpiry('2026-09-18T08:00:01Z', now)).toBe('any second now')
  })

  it('says so when the bin is already gone', () => {
    expect(untilExpiry('2026-09-18T08:00:00Z', now)).toBe('expired')
    expect(untilExpiry('2026-09-11T08:00:00Z', now)).toBe('expired')
  })

  it('stays quiet on a value it cannot read', () => {
    expect(untilExpiry('', now)).toBe('')
    expect(formatInstant('not a time')).toBe('not a time')
  })
})
