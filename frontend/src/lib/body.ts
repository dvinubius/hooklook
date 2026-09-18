/** Captured request bodies: exact bytes in, readable text out.
 *
 * A captured body is bytes someone else chose. Three rules hold throughout.
 * The bytes are never altered — what is shown is what was stored. Nothing here
 * ever produces markup: formatting returns strings and highlighting returns
 * token *data*, which the components render through text interpolation, so a
 * body containing `<script>` is characters on a page and nothing else. And a
 * body we cannot make sense of is reported as such, with the raw view intact,
 * rather than guessed at. */

export type BodyKind = 'empty' | 'text' | 'binary'
export type BodyFormat = 'text' | 'json' | 'xml'

export interface DecodedBody {
  bytes: Uint8Array
  /** Exact byte count — the list's `bodySizeKiB` is rounded up to whole KiB. */
  size: number
  kind: BodyKind
  /** The encoding the text was actually decoded with, named for the reader. */
  encoding: string
  /** True when the declared or assumed encoding did not fit and a fallback
   *  was used, which is worth saying rather than hiding. */
  fellBack: boolean
  /** Exact decoded text. Empty for an empty or binary body. */
  text: string
  format: BodyFormat
  /** Pretty-printed text, or null when there was nothing to format or the
   *  content did not parse. */
  formatted: string | null
  /** Why formatting did not happen, when the body claimed a format. */
  formatNote: string
}

/** Go encodes `[]byte` as base64 in JSON, and sends `null` for an empty body. */
export function decodeBase64(raw: string | null | undefined): Uint8Array {
  if (!raw) return new Uint8Array(0)
  const binary = atob(raw)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i)
  return bytes
}

/** `text/plain; charset=iso-8859-1` → `iso-8859-1`. */
export function charsetOf(contentType: string): string {
  const match = /;\s*charset\s*=\s*"?([^";]+)"?/i.exec(contentType)
  return match ? match[1]!.trim().toLowerCase() : ''
}

function mediaTypeOf(contentType: string): string {
  return contentType.split(';')[0]!.trim().toLowerCase()
}

/** A NUL byte is decisive: no text encoding this application shows uses one.
 *  Otherwise it takes a real density of control bytes, so that one stray
 *  0x01 in a log line does not turn a readable body into a hex dump. */
export function looksBinary(bytes: Uint8Array): boolean {
  if (bytes.length === 0) return false
  let controls = 0
  for (const byte of bytes) {
    if (byte === 0) return true
    if ((byte < 0x09 || (byte > 0x0d && byte < 0x20) || byte === 0x7f)) controls += 1
  }
  return controls / bytes.length > 0.05
}

function decodeWith(bytes: Uint8Array, label: string): string | null {
  try {
    return new TextDecoder(label, { fatal: true }).decode(bytes)
  } catch {
    return null
  }
}

/** Decoding is tried in declared order and the result says which one won.
 *  `windows-1252` is last because it maps every byte and so always succeeds:
 *  it is the honest "these bytes are not UTF-8" answer, not a guess dressed
 *  up as a fact. */
function decodeText(bytes: Uint8Array, declared: string): { text: string; encoding: string; fellBack: boolean } {
  const attempts = declared ? [declared, 'utf-8', 'windows-1252'] : ['utf-8', 'windows-1252']
  for (const [index, label] of attempts.entries()) {
    const text = decodeWith(bytes, label)
    if (text !== null) return { text, encoding: label, fellBack: index > 0 }
  }
  // Unreachable in practice: windows-1252 decodes any byte sequence.
  return { text: new TextDecoder('windows-1252').decode(bytes), encoding: 'windows-1252', fellBack: true }
}

function sniffFormat(contentType: string, text: string): BodyFormat {
  const media = mediaTypeOf(contentType)
  if (media === 'application/json' || media.endsWith('+json')) return 'json'
  if (media === 'text/xml' || media === 'application/xml' || media.endsWith('+xml')) return 'xml'
  if (media !== '' && media !== 'text/plain') return 'text'
  const start = text.trimStart()[0]
  if (start === '{' || start === '[') return 'json'
  if (start === '<') return 'xml'
  return 'text'
}

/** Above this, the body is shown raw. Formatting and highlighting both walk
 *  the whole string, and a reader is not reading a megabyte by eye anyway. */
export const formatLimitBytes = 256 * 1024

export function describeBody(rawBody: string | null | undefined, contentType: string): DecodedBody {
  const bytes = decodeBase64(rawBody)
  const empty: DecodedBody = {
    bytes,
    size: bytes.length,
    kind: 'empty',
    encoding: '',
    fellBack: false,
    text: '',
    format: 'text',
    formatted: null,
    formatNote: '',
  }
  if (bytes.length === 0) return empty
  if (looksBinary(bytes)) return { ...empty, kind: 'binary' }

  const { text, encoding, fellBack } = decodeText(bytes, charsetOf(contentType))
  const format = sniffFormat(contentType, text)
  const decoded: DecodedBody = { ...empty, kind: 'text', encoding, fellBack, text, format }

  if (format === 'text') return decoded
  if (bytes.length > formatLimitBytes) {
    return { ...decoded, formatNote: 'Too large to format; showing the raw body.' }
  }
  try {
    return { ...decoded, formatted: format === 'json' ? formatJSON(text) : formatXML(text) }
  } catch (cause) {
    const detail = cause instanceof Error ? cause.message : 'it could not be parsed'
    return { ...decoded, formatNote: `This body is not valid ${format.toUpperCase()}: ${detail}.` }
  }
}

export function formatJSON(text: string): string {
  return JSON.stringify(JSON.parse(text), null, 2)
}

// ---- XML --------------------------------------------------------------
// A small, dependency-free formatter. Captured bytes are deliberately never
// handed to a DOM parser: this walks the text and prints it back, and the
// only judgement it makes is about indentation.

type Piece = { kind: 'open' | 'close' | 'self' | 'meta' | 'text'; text: string; name: string }

const xmlTag = /<!--[\s\S]*?-->|<[!?][\s\S]*?>|<\/[^>]*>|<[^>]*>/g

function tagName(tag: string): string {
  const match = /^<\/?\s*([^\s/>]+)/.exec(tag)
  return match ? match[1]! : ''
}

function classify(tag: string): Piece {
  const name = tagName(tag)
  if (tag.startsWith('<!') || tag.startsWith('<?')) return { kind: 'meta', text: tag, name }
  if (tag.startsWith('</')) return { kind: 'close', text: tag, name }
  if (tag.endsWith('/>')) return { kind: 'self', text: tag, name }
  return { kind: 'open', text: tag, name }
}

function pieces(text: string): Piece[] {
  const found: Piece[] = []
  let cursor = 0
  xmlTag.lastIndex = 0
  for (let match = xmlTag.exec(text); match !== null; match = xmlTag.exec(text)) {
    const between = text.slice(cursor, match.index).trim()
    if (between !== '') found.push({ kind: 'text', text: between, name: '' })
    found.push(classify(match[0]))
    cursor = match.index + match[0].length
  }
  const tail = text.slice(cursor).trim()
  if (tail !== '') found.push({ kind: 'text', text: tail, name: '' })
  return found
}

export function formatXML(text: string): string {
  const parts = pieces(text)
  if (!parts.some((piece) => piece.kind !== 'text')) throw new Error('no elements were found')

  const lines: string[] = []
  const open: string[] = []
  const indent = (): string => '  '.repeat(open.length)

  for (let i = 0; i < parts.length; i += 1) {
    const piece = parts[i]!
    if (piece.kind === 'close') {
      const expected = open.pop()
      if (expected === undefined) throw new Error(`</${piece.name}> closes an element that was never opened`)
      if (expected !== piece.name) throw new Error(`<${expected}> is closed by </${piece.name}>`)
      lines.push(indent() + piece.text)
      continue
    }
    if (piece.kind === 'open') {
      // `<name>value</name>` stays on one line: splitting it would suggest
      // whitespace the body does not contain.
      const value = parts[i + 1]
      const closer = parts[i + 2]
      if (value?.kind === 'text' && closer?.kind === 'close' && closer.name === piece.name) {
        lines.push(indent() + piece.text + value.text + closer.text)
        i += 2
        continue
      }
      lines.push(indent() + piece.text)
      open.push(piece.name)
      continue
    }
    lines.push(indent() + piece.text)
  }

  if (open.length > 0) throw new Error(`<${open[open.length - 1]}> is never closed`)
  return lines.join('\n')
}

// ---- highlighting -----------------------------------------------------
// The brand's code surfaces carry a brightness ramp, not a color palette:
// emphasis for names, body for values, recede for punctuation. These are
// produced as data and interpolated as text, so nothing captured is ever
// parsed as markup by the browser.

export type Tier = 'emphasis' | 'body' | 'recede'

export interface Token {
  text: string
  tier: Tier
}

const jsonScan =
  /("(?:[^"\\]|\\[\s\S])*")(\s*:)|("(?:[^"\\]|\\[\s\S])*")|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)|(true|false|null)|([{}[\],:])/g

function jsonTokens(text: string): Token[] {
  const tokens: Token[] = []
  let cursor = 0
  jsonScan.lastIndex = 0
  for (let match = jsonScan.exec(text); match !== null; match = jsonScan.exec(text)) {
    if (match.index > cursor) tokens.push({ text: text.slice(cursor, match.index), tier: 'recede' })
    const [whole, key, colon, str, num, literal] = match
    if (key !== undefined) {
      tokens.push({ text: key, tier: 'emphasis' })
      tokens.push({ text: colon!, tier: 'recede' })
    } else if (str !== undefined || num !== undefined || literal !== undefined) {
      tokens.push({ text: whole, tier: 'body' })
    } else {
      tokens.push({ text: whole, tier: 'recede' })
    }
    cursor = match.index + whole.length
  }
  if (cursor < text.length) tokens.push({ text: text.slice(cursor), tier: 'recede' })
  return tokens
}

function xmlTagTokens(tag: string): Token[] {
  if (tag.startsWith('<!') || tag.startsWith('<?')) return [{ text: tag, tier: 'recede' }]
  const match = /^(<\/?)([^\s/>]+)([\s\S]*?)(\/?>)$/.exec(tag)
  if (!match) return [{ text: tag, tier: 'recede' }]
  const [, opener, name, attributes, closer] = match
  const tokens: Token[] = [
    { text: opener!, tier: 'recede' },
    { text: name!, tier: 'emphasis' },
  ]
  if (attributes !== '') tokens.push({ text: attributes!, tier: 'body' })
  tokens.push({ text: closer!, tier: 'recede' })
  return tokens
}

function xmlTokens(text: string): Token[] {
  const tokens: Token[] = []
  let cursor = 0
  xmlTag.lastIndex = 0
  for (let match = xmlTag.exec(text); match !== null; match = xmlTag.exec(text)) {
    if (match.index > cursor) tokens.push({ text: text.slice(cursor, match.index), tier: 'body' })
    tokens.push(...xmlTagTokens(match[0]))
    cursor = match.index + match[0].length
  }
  if (cursor < text.length) tokens.push({ text: text.slice(cursor), tier: 'body' })
  return tokens
}

export function highlight(text: string, format: BodyFormat): Token[] {
  if (text === '') return []
  if (format === 'text' || text.length > formatLimitBytes) return [{ text, tier: 'body' }]
  return format === 'json' ? jsonTokens(text) : xmlTokens(text)
}

// ---- binary -----------------------------------------------------------

/** Offset, sixteen bytes, and the printable ASCII beside them — the plain
 *  way to look at bytes that are not text. */
export function hexDump(
  bytes: Uint8Array,
  limit = 4096,
): { lines: string[]; shown: number; truncated: boolean } {
  const shown = bytes.subarray(0, limit)
  const lines: string[] = []
  for (let offset = 0; offset < shown.length; offset += 16) {
    const row = shown.subarray(offset, offset + 16)
    const hex = [...row].map((byte) => byte.toString(16).padStart(2, '0')).join(' ').padEnd(47, ' ')
    const ascii = [...row]
      .map((byte) => (byte >= 0x20 && byte < 0x7f ? String.fromCharCode(byte) : '.'))
      .join('')
    lines.push(`${offset.toString(16).padStart(8, '0')}  ${hex}  |${ascii}|`)
  }
  return { lines, shown: shown.length, truncated: bytes.length > shown.length }
}

/** Byte counts in the units the reader reads them in. The list's rounded
 *  `bodySizeKiB` is the server's; this is exact. */
export function formatBytes(size: number): string {
  if (size < 1024) return `${size} ${size === 1 ? 'byte' : 'bytes'}`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KiB`
  return `${(size / (1024 * 1024)).toFixed(1)} MiB`
}
