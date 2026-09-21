/** Reading the client address back out of a captured header.
 *
 * Nothing about this is stored, so every case here is a header shape a real
 * sender or proxy can produce. */

import { describe, expect, it } from 'vitest'
import { clientIp, forwardedHops } from '../lib/headers'

describe('client ip from X-Forwarded-For', () => {
  it('reads the header Go canonicalized', () => {
    expect(clientIp({ 'X-Forwarded-For': ['203.0.113.7'] })).toBe('203.0.113.7')
  })

  it('finds the header whatever its case', () => {
    expect(clientIp({ 'x-forwarded-for': ['203.0.113.7'] })).toBe('203.0.113.7')
    expect(clientIp({ 'X-FORWARDED-FOR': ['203.0.113.7'] })).toBe('203.0.113.7')
  })

  it('is empty when the capture never passed the proxy', () => {
    expect(clientIp({ 'Content-Type': ['application/json'] })).toBe('')
    expect(clientIp({})).toBe('')
  })

  // The last hop is the address our own ingress accepted the connection
  // from; the ones before it are whatever the caller chose to send.
  it('takes the hop the proxy appended, not the one the caller claimed', () => {
    expect(clientIp({ 'X-Forwarded-For': ['1.2.3.4, 203.0.113.7'] })).toBe('203.0.113.7')
  })

  it('treats a repeated header the way HTTP does, as one list in order', () => {
    expect(clientIp({ 'X-Forwarded-For': ['1.2.3.4', '203.0.113.7'] })).toBe('203.0.113.7')
    expect(forwardedHops({ 'X-Forwarded-For': ['1.2.3.4, 10.0.0.1', '203.0.113.7'] })).toEqual([
      '1.2.3.4',
      '10.0.0.1',
      '203.0.113.7',
    ])
  })

  it('ignores the spacing a sender chose, and empty entries', () => {
    expect(clientIp({ 'X-Forwarded-For': ['  1.2.3.4 ,   203.0.113.7  '] })).toBe('203.0.113.7')
    expect(clientIp({ 'X-Forwarded-For': ['1.2.3.4, , '] })).toBe('1.2.3.4')
    expect(clientIp({ 'X-Forwarded-For': [''] })).toBe('')
  })

  it('reads IPv6 as the single value it is', () => {
    expect(clientIp({ 'X-Forwarded-For': ['2001:db8::1'] })).toBe('2001:db8::1')
  })
})
