/** Reading meaning out of the headers a capture arrived with.
 *
 * Nothing here is stored: every value is derived from the headers already
 * captured, so there is no column and no capture field behind it. */

/** Header names arrive as the sender wrote them. Go canonicalizes what it
 *  receives, so in practice the key is `X-Forwarded-For` — but the stored map
 *  is plain JSON and nothing downstream enforces that, so every lookup here
 *  is case-insensitive. Repeats under different spellings are joined in the
 *  order they appear, the way HTTP treats a header sent more than once. */
function headerValues(headers: Record<string, string[]>, name: string): string[] {
  const wanted = name.toLowerCase()
  const found: string[] = []
  for (const [key, values] of Object.entries(headers)) {
    if (key.toLowerCase() === wanted) found.push(...values)
  }
  return found
}

/** The hops in `X-Forwarded-For`, oldest first. A repeated header and a
 *  comma-separated one mean the same thing, so both flatten to one list. */
export function forwardedHops(headers: Record<string, string[]>): string[] {
  return headerValues(headers, 'x-forwarded-for')
    .flatMap((value) => value.split(','))
    .map((hop) => hop.trim())
    .filter((hop) => hop !== '')
}

/** The client as our own ingress saw it: the LAST hop, not the first.
 *
 *  Caddy appends the address it accepted the connection from, so a caller who
 *  sends `X-Forwarded-For: 1.2.3.4` arrives as `1.2.3.4, <their real
 *  address>`. The first hop is therefore whatever the caller chose to claim,
 *  and the last is the only one this deployment observed. Ordinary traffic
 *  has exactly one hop and the two readings agree; they part only when
 *  someone is lying, which is when it matters.
 *
 *  This holds because Caddy is the only public ingress and Go cannot be
 *  reached directly. Put another proxy or a CDN in front and the trusted hop
 *  moves further left — revisit this then. Empty when the header is absent. */
export function clientIp(headers: Record<string, string[]>): string {
  const hops = forwardedHops(headers)
  return hops.length === 0 ? '' : hops[hops.length - 1]!
}
