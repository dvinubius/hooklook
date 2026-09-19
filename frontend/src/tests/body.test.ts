import { describe, expect, it } from 'vitest'
import {
  charsetOf,
  decodeBase64,
  describeBody,
  formatBytes,
  formatXML,
  hexDump,
  highlight,
  looksBinary,
} from '../lib/body'

function base64(bytes: number[] | string): string {
  const data = typeof bytes === 'string' ? [...new TextEncoder().encode(bytes)] : bytes
  return btoa(String.fromCharCode(...data))
}

describe('decodeBase64', () => {
  it('returns the exact bytes, and nothing for an absent body', () => {
    expect([...decodeBase64(null)]).toEqual([])
    expect([...decodeBase64('')]).toEqual([])
    expect([...decodeBase64(base64([0, 1, 255, 128]))]).toEqual([0, 1, 255, 128])
  })
})

describe('charsetOf', () => {
  it('reads the declared encoding, quoted or not', () => {
    expect(charsetOf('text/plain; charset=ISO-8859-1')).toBe('iso-8859-1')
    expect(charsetOf('application/json;charset="utf-8"')).toBe('utf-8')
    expect(charsetOf('application/json')).toBe('')
  })
})

describe('looksBinary', () => {
  it('treats a NUL byte as decisive and tolerates a stray control byte', () => {
    expect(looksBinary(new Uint8Array([]))).toBe(false)
    expect(looksBinary(new TextEncoder().encode('hello'))).toBe(false)
    expect(looksBinary(new Uint8Array([0x68, 0x00, 0x69]))).toBe(true)
    // One 0x01 in a long line of text is not a hex dump.
    expect(looksBinary(new Uint8Array([0x01, ...new TextEncoder().encode('a'.repeat(100))]))).toBe(false)
    expect(looksBinary(new Uint8Array([0x01, 0x02, 0x03, 0x04]))).toBe(true)
  })
})

describe('describeBody', () => {
  it('reports an empty body as empty rather than as empty text', () => {
    const body = describeBody(null, 'application/json')
    expect(body.kind).toBe('empty')
    expect(body.size).toBe(0)
  })

  it('pretty-prints JSON and keeps the exact raw text beside it', () => {
    const body = describeBody(base64('{"b":1,"a":[2,3]}'), 'application/json')
    expect(body.kind).toBe('text')
    expect(body.format).toBe('json')
    expect(body.text).toBe('{"b":1,"a":[2,3]}')
    expect(body.formatted).toBe('{\n  "b": 1,\n  "a": [\n    2,\n    3\n  ]\n}')
    expect(body.formatNote).toBe('')
  })

  it('explains malformed JSON locally and keeps the raw view', () => {
    const body = describeBody(base64('{"a":'), 'application/json')
    expect(body.formatted).toBeNull()
    expect(body.formatNote).toContain('not valid JSON')
    expect(body.text).toBe('{"a":')
  })

  it('sniffs a format when the content type does not name one', () => {
    expect(describeBody(base64('  [1,2]'), '').format).toBe('json')
    expect(describeBody(base64('<a/>'), 'text/plain').format).toBe('xml')
    expect(describeBody(base64('plain words'), '').format).toBe('text')
  })

  it('falls back to a byte-complete encoding and says that it did', () => {
    // 0xFF is not valid UTF-8; the body is still text and must stay readable.
    const body = describeBody(base64([0x63, 0x61, 0x66, 0xe9]), 'text/plain')
    expect(body.kind).toBe('text')
    expect(body.encoding).toBe('windows-1252')
    expect(body.fellBack).toBe(true)
    expect(body.text).toBe('café')
  })

  it('honours a declared encoding without falling back', () => {
    const body = describeBody(base64([0x63, 0x61, 0x66, 0xe9]), 'text/plain; charset=iso-8859-1')
    expect(body.encoding).toBe('iso-8859-1')
    expect(body.fellBack).toBe(false)
  })

  it('keeps valid UTF-8 as UTF-8', () => {
    const body = describeBody(base64('héllo ✓'), 'text/plain')
    expect(body.encoding).toBe('utf-8')
    expect(body.fellBack).toBe(false)
    expect(body.text).toBe('héllo ✓')
  })

  it('calls a body with NUL bytes binary and decodes no text from it', () => {
    const body = describeBody(base64([0x89, 0x50, 0x4e, 0x47, 0x00, 0x1a]), 'image/png')
    expect(body.kind).toBe('binary')
    expect(body.text).toBe('')
    expect(body.size).toBe(6)
  })

  it('never turns a captured body into markup', () => {
    const body = describeBody(base64('<script>alert(1)</script>'), 'text/html')
    expect(body.text).toBe('<script>alert(1)</script>')
    expect(highlight(body.text, body.format).map((token) => token.text).join('')).toBe(body.text)
  })
})

describe('formatXML', () => {
  it('indents elements and keeps a leaf value on its own line', () => {
    expect(formatXML('<a><b>1</b><c/></a>')).toBe('<a>\n  <b>1</b>\n  <c/>\n</a>')
  })

  it('keeps declarations and comments', () => {
    expect(formatXML('<?xml version="1.0"?><a/>')).toBe('<?xml version="1.0"?>\n<a/>')
  })

  it('refuses content it cannot account for', () => {
    expect(() => formatXML('<a></b>')).toThrow(/closed by/)
    expect(() => formatXML('<a>')).toThrow(/never closed/)
    expect(() => formatXML('</a>')).toThrow(/never opened/)
    expect(() => formatXML('no tags here')).toThrow(/no elements/)
  })
})

describe('highlight', () => {
  it('reproduces the input exactly, whatever the tiers', () => {
    const json = '{\n  "a": "<b>",\n  "n": 1\n}'
    expect(highlight(json, 'json').map((token) => token.text).join('')).toBe(json)
    const xml = '<a href="x">t &amp; u</a>'
    expect(highlight(xml, 'xml').map((token) => token.text).join('')).toBe(xml)
  })

  it('colors JSON keys as names and every JSON value as a value', () => {
    const tiers = new Map(
      highlight('{"a": "v", "n": 1, "b": true}', 'json').map((t) => [t.text, t.tier]),
    )
    expect(tiers.get('"a"')).toBe('name')
    expect(tiers.get('"v"')).toBe('value')
    expect(tiers.get('1')).toBe('value')
    expect(tiers.get('true')).toBe('value')
    expect(tiers.get('{')).toBe('recede')
  })

  it('colors XML element and attribute names as names, attribute values and text as values', () => {
    const xml = highlight(`<item id="3" kind='a'>v</item>`, 'xml')
    const tiers = new Map(xml.map((t) => [t.text, t.tier]))
    expect(tiers.get('item')).toBe('name')
    expect(tiers.get('id')).toBe('name')
    expect(tiers.get('"3"')).toBe('value')
    expect(tiers.get(`'a'`)).toBe('value')
    expect(tiers.get('v')).toBe('value')
    expect(tiers.get('<')).toBe('recede')
    expect(tiers.get('=')).toBe('recede')
  })

  it('leaves plain text as one run', () => {
    expect(highlight('hello', 'text')).toEqual([{ text: 'hello', tier: 'body' }])
    expect(highlight('', 'json')).toEqual([])
  })
})

describe('hexDump', () => {
  it('lays out offset, bytes and printable ASCII', () => {
    const { lines, truncated } = hexDump(new Uint8Array([0x68, 0x69, 0x00]))
    expect(lines[0]).toBe(
      '00000000  68 69 00                                         |hi.|',
    )
    expect(truncated).toBe(false)
  })

  it('stops at the limit and says how far it got', () => {
    const { lines, shown, truncated } = hexDump(new Uint8Array(64), 32)
    expect(lines).toHaveLength(2)
    expect(shown).toBe(32)
    expect(truncated).toBe(true)
  })
})

describe('formatBytes', () => {
  it('uses the unit the number deserves', () => {
    expect(formatBytes(0)).toBe('0 bytes')
    expect(formatBytes(1)).toBe('1 byte')
    expect(formatBytes(2048)).toBe('2.0 KiB')
    expect(formatBytes(3 * 1024 * 1024)).toBe('3.0 MiB')
  })
})
