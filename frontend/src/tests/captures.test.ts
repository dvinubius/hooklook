/** The seam between the two halves.
 *
 * Every `rawBody` below was copied out of a running server's detail response
 * after sending that body to a real bin, so these assert what the application
 * will actually be handed rather than what a hand-written fixture assumes. */

import { describe, expect, it } from 'vitest'
import { describeBody, highlight } from '../lib/body'

const captured = {
  json: {
    contentType: 'application/json',
    rawBody: 'eyJiIjoxLCJhIjpbIjxzY3JpcHQ+YWxlcnQoMSk8L3NjcmlwdD4iXX0=',
  },
  xml: {
    contentType: 'application/xml',
    rawBody: 'PG9yZGVyIGlkPSI3Ij48aXRlbT53aWRnZXQ8L2l0ZW0+PC9vcmRlcj4=',
  },
  utf8: { contentType: 'text/plain', rawBody: 'aMOpbGxvIOKckyDihrM=' },
  latin1: { contentType: 'text/plain', rawBody: 'Y2Fm6SBuYe92ZQ==' },
  png: { contentType: 'image/png', rawBody: 'iVBORw0KGgoAAAANSUhEUg==' },
  malformed: { contentType: 'application/json', rawBody: 'eyJhIjo=' },
  empty: { contentType: '', rawBody: '' },
}

describe('bodies as the server really sends them', () => {
  it('pretty-prints a JSON capture without letting its markup become markup', () => {
    const body = describeBody(captured.json.rawBody, captured.json.contentType)
    expect(body.kind).toBe('text')
    expect(body.format).toBe('json')
    expect(body.text).toBe('{"b":1,"a":["<script>alert(1)</script>"]}')
    expect(body.formatted).toContain('"<script>alert(1)</script>"')
    // The `<script>` survives as characters in a token, never as an element.
    const tokens = highlight(body.formatted!, 'json')
    expect(tokens.map((token) => token.text).join('')).toBe(body.formatted)
    expect(tokens.some((token) => token.text.includes('<script>'))).toBe(true)
  })

  it('indents an XML capture and names its elements', () => {
    const body = describeBody(captured.xml.rawBody, captured.xml.contentType)
    expect(body.format).toBe('xml')
    expect(body.formatted).toBe('<order id="7">\n  <item>widget</item>\n</order>')
    const tokens = highlight(body.formatted!, 'xml')
    expect(tokens.map((token) => token.text).join('')).toBe(body.formatted)
    // Names of data take the name tier, the data itself the value tier.
    const tiers = new Map(tokens.map((t) => [t.text, t.tier]))
    expect(tiers.get('order')).toBe('name')
    expect(tiers.get('item')).toBe('name')
    expect(tiers.get('id')).toBe('name')
    expect(tiers.get('"7"')).toBe('value')
    expect(tiers.get('widget')).toBe('value')
    expect(tiers.get('<')).toBe('recede')
  })

  it('reads a UTF-8 capture, brand glyphs and all', () => {
    const body = describeBody(captured.utf8.rawBody, captured.utf8.contentType)
    expect(body.encoding).toBe('utf-8')
    expect(body.fellBack).toBe(false)
    expect(body.text).toBe('héllo ✓ ↳')
  })

  it('still reads a capture that is not UTF-8, and says which encoding it used', () => {
    const body = describeBody(captured.latin1.rawBody, captured.latin1.contentType)
    expect(body.kind).toBe('text')
    expect(body.encoding).toBe('windows-1252')
    expect(body.fellBack).toBe(true)
    expect(body.text).toBe('café naïve')
  })

  it('treats a PNG header as bytes, not text', () => {
    const body = describeBody(captured.png.rawBody, captured.png.contentType)
    expect(body.kind).toBe('binary')
    expect(body.size).toBe(16)
    expect([...body.bytes.subarray(0, 4)]).toEqual([0x89, 0x50, 0x4e, 0x47])
  })

  it('keeps a malformed JSON capture readable and explains the failure', () => {
    const body = describeBody(captured.malformed.rawBody, captured.malformed.contentType)
    expect(body.text).toBe('{"a":')
    expect(body.formatted).toBeNull()
    expect(body.formatNote).toContain('not valid JSON')
  })

  it('reports a request with no body as having none', () => {
    // Go sends an absent body as an empty base64 string, not as null.
    const body = describeBody(captured.empty.rawBody, captured.empty.contentType)
    expect(body.kind).toBe('empty')
    expect(body.size).toBe(0)
  })
})
