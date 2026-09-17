import { describe, expect, it } from 'vitest'
import { captureUrl, inviteUrl, pagePath, parseLocation } from '../lib/location'

const origin = 'https://hooklook.example'

describe('parseLocation', () => {
  it('reads the bin code from a bin page', () => {
    expect(parseLocation(`${origin}/bins/brave-otter-19472841`, origin)).toEqual({
      origin,
      code: 'brave-otter-19472841',
      requestId: null,
      invite: '',
    })
  })

  it('reads the selected request from the detail URL a capture reports', () => {
    const page = parseLocation(`${origin}/bins/brave-otter-19472841/requests/42`, origin)
    expect(page.code).toBe('brave-otter-19472841')
    expect(page.requestId).toBe('42')
  })

  it('carries the invitation at either page depth', () => {
    expect(parseLocation(`${origin}/bins/code-1?invite=abc123`, origin).invite).toBe('abc123')
    expect(parseLocation(`${origin}/bins/code-1/requests/7?invite=abc123`, origin).invite).toBe(
      'abc123',
    )
  })

  it('tolerates a trailing slash and percent-encoding', () => {
    const page = parseLocation(`${origin}/bins/od%2Fd/requests/a%20b/`, origin)
    expect(page.code).toBe('od/d')
    expect(page.requestId).toBe('a b')
  })

  it('reports no bin for a path this application was never served at', () => {
    expect(parseLocation(`${origin}/`, origin).code).toBe('')
    expect(parseLocation(`${origin}/b/some-code`, origin).code).toBe('')
    expect(parseLocation(`${origin}/bins/code/requests/7/extra`, origin).code).toBe('')
  })

  it('keeps the origin the browser is really on', () => {
    expect(parseLocation('http://localhost:5173/bins/code-1', 'http://localhost:5173').origin).toBe(
      'http://localhost:5173',
    )
  })
})

describe('URL building', () => {
  it('builds the capture URL on the current origin', () => {
    expect(captureUrl('brave-otter-19472841', origin)).toBe(
      `${origin}/b/brave-otter-19472841`,
    )
  })

  it('keeps the invitation in the page path and share link', () => {
    expect(pagePath('code-1', '42', 'abc123')).toBe('/bins/code-1/requests/42?invite=abc123')
    expect(pagePath('code-1', null, '')).toBe('/bins/code-1')
    expect(inviteUrl('code-1', 'abc123', origin)).toBe(`${origin}/bins/code-1?invite=abc123`)
  })
})
