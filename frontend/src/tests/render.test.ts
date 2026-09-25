/** What the components actually put on the page.
 *
 * Rendered in Node through Vue's own server renderer, so these assert real
 * output — above all that a captured body reaches the page as characters and
 * never as markup, and that a guest is shown no control they cannot use. */

import { describe, expect, it } from 'vitest'
import { createSSRApp, h, type Component } from 'vue'
import { renderToString } from 'vue/server-renderer'
import BodyView from '../components/BodyView.vue'
import BinUnavailable from '../components/BinUnavailable.vue'
import HeadersTable from '../components/HeadersTable.vue'
import RequestDetailView from '../components/RequestDetail.vue'
import CapacityGauge from '../components/CapacityGauge.vue'
import RequestList from '../components/RequestList.vue'
import SelectMenu from '../components/SelectMenu.vue'
import { describeBody } from '../lib/body'
import ServiceFull from '../components/ServiceFull.vue'
import type { BinCapacity, RequestDetail, RequestSummary } from '../types'

function render(component: Component, props: Record<string, unknown>): Promise<string> {
  return renderToString(createSSRApp({ render: () => h(component, props) }))
}

/** The text a reader actually sees, with the rendered markup taken back off.
 *  Comparing this to the body proves the captured characters survived intact
 *  while every one of them stayed inside a text node. */
function textOf(html: string): string {
  return html
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/<[^>]*>/g, '')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&')
}

const summary: RequestSummary = {
  id: '12',
  method: 'POST',
  path: '/orders/42',
  rawQuery: 'retry=1',
  receivedAt: '2026-09-18T08:00:00Z',
  contentType: 'application/json',
  bodySizeKiB: 1,
  headerCount: 4,
}

/** Room to spare unless a test says otherwise. */
function capacity(fields: Partial<BinCapacity> = {}): BinCapacity {
  return {
    requestCount: 1,
    requestLimit: 500,
    bodyBytesUsed: 1_000,
    bodyBytesLimit: 100_000_000,
    requestsFull: false,
    bodyBytesFull: false,
    full: false,
    ...fields,
  }
}

const listProps = {
  summaries: [summary],
  loading: false,
  loaded: true,
  stream: 'live',
  error: '',
  selectedId: null,
  owner: true,
  deleting: false,
}

describe('BodyView', () => {
  it('escapes a captured body instead of letting it become markup', async () => {
    const body = describeBody(
      btoa('{"a":"<script>alert(1)</script>","b":"<img src=x onerror=1>"}'),
      'application/json',
    )
    const html = await render(BodyView, { body })

    // No element and no event-handler attribute came out of the body...
    expect(html).not.toMatch(/<script/i)
    expect(html).not.toMatch(/<img/i)
    expect(html).not.toMatch(/<[a-z][^>]*\son\w+=/i)
    // ...and yet every character of it is on the page, exactly as captured.
    expect(textOf(html)).toContain('"<script>alert(1)</script>"')
    expect(textOf(html)).toContain('"<img src=x onerror=1>"')
  })

  it('offers raw and formatted views, formatted by default, and a copy control', async () => {
    const html = await render(BodyView, { body: describeBody(btoa('{"a":1}'), 'application/json') })
    expect(html).toMatch(/aria-pressed="false"[^>]*>\s*raw\s*</)
    expect(html).toMatch(/aria-pressed="true"[^>]*>\s*formatted\s*</)
    expect(html).toContain('aria-label="Copy raw body"')
    expect(html).not.toContain('Copy body<')
  })

  it('has no view choice when there is nothing to format', async () => {
    const html = await render(BodyView, { body: describeBody(btoa('plain words'), 'text/plain') })
    expect(html).not.toContain('aria-pressed')
    expect(html).toContain('aria-label="Copy raw body"')
  })

  it('escapes an XML capture too, tags and all', async () => {
    const body = describeBody(btoa('<a><b>t &amp; u</b></a>'), 'application/xml')
    const html = await render(BodyView, { body })
    expect(html).not.toMatch(/<a[ >]/)
    expect(html).not.toMatch(/<b>/)
    // The pretty-printed XML, entity and all, as characters on the page.
    expect(textOf(html)).toContain('<a>\n  <b>t &amp; u</b>\n</a>')
  })

  it('says a body is empty rather than showing an empty box', async () => {
    const html = await render(BodyView, { body: describeBody('', '') })
    expect(html).toContain('no body')
  })

  it('shows bytes as a hex dump and names the encoding of text', async () => {
    const binary = await render(BodyView, {
      body: describeBody(btoa('\x89PNG\x00\x01\x02'), 'image/png'),
    })
    expect(binary).toContain('binary')
    expect(binary).toContain('89 50 4e 47')

    const text = await render(BodyView, { body: describeBody(btoa('caf\xe9'), 'text/plain') })
    expect(text).toContain('windows-1252')
    expect(text).toContain('not valid UTF-8')
  })

  it('keeps the raw view and explains itself when formatting fails', async () => {
    const html = await render(BodyView, { body: describeBody(btoa('{"a":'), 'application/json') })
    expect(html).toContain('not valid JSON')
    expect(textOf(html)).toContain('{"a":')
  })
})

describe('HeadersTable', () => {
  it('shows a redacted value as stored and does not pretend to recover it', async () => {
    const html = await render(HeadersTable, {
      headers: { Authorization: ['[REDACTED]'], 'Content-Type': ['application/json'] },
    })
    expect(html).toContain('[REDACTED]')
    expect(html).toContain('application/json')
  })

  it('escapes a header value as readily as a body', async () => {
    const html = await render(HeadersTable, { headers: { 'X-Note': ['<b>hi</b>'] } })
    expect(html).not.toMatch(/<b>hi<\/b>/)
    expect(textOf(html)).toContain('<b>hi</b>')
  })

  it('gives each header a labelled copy control', async () => {
    const html = await render(HeadersTable, { headers: { Accept: ['text/html', 'application/json'] } })
    expect(html).toContain('aria-label="Copy Accept header"')
  })

  it('keeps a long name whole in the page, however it is cut on screen', async () => {
    const name = 'X-Very-Long-Vendor-Specific-Signature-Header'
    const html = await render(HeadersTable, { headers: { [name]: ['v'] } })
    expect(html).toMatch(new RegExp(`<dt class="name"[^>]*>${name}</dt>`))
  })

  it('says so when nothing was stored', async () => {
    const html = await render(HeadersTable, { headers: {} })
    expect(html).toContain('no headers were stored')
  })
})

describe('RequestList', () => {
  it('shows each capture with its method, path and query', async () => {
    const html = await render(RequestList, listProps)
    expect(html).toContain('POST')
    expect(html).toContain('/orders/42')
    expect(html).toContain('?retry=1')
    // The query hangs off the end of the path with nothing between them:
    // the two are separate nodes, so a stray newline in the template would
    // put a space there and read as a different address.
    expect(textOf(html)).toContain('/orders/42?retry=1')
  })

  it('counts nothing while no filter is on', async () => {
    // The filters are the list's own state, so a server render can only see
    // the unfiltered case; what a filter leaves is counted in the browser.
    expect(textOf(await render(RequestList, listProps))).not.toContain('Results:')
  })

  it('distinguishes no-requests-yet from nothing-matching', async () => {
    const empty = await render(RequestList, { ...listProps, summaries: [] })
    expect(empty).toContain('no requests captured yet')

    const loading = await render(RequestList, { ...listProps, loading: true, loaded: false, summaries: [] })
    expect(loading).toContain('loading captured requests')
  })

  it('shows a live stream as streaming with a green dot, and anything else as connecting with a grey one', async () => {
    const live = await render(RequestList, listProps)
    expect(live).toContain('class="dot live"')
    expect(textOf(live)).toContain('streaming')
    for (const stream of ['connecting', 'reconnecting', 'closed'] as const) {
      const down = await render(RequestList, { ...listProps, stream })
      expect(textOf(down)).toContain('connecting…')
      expect(down).toContain('class="dot waiting"')
      expect(down).not.toContain('class="dot live"')
    }
  })

  it('offers a reload when the list itself could not be read', async () => {
    const html = await render(RequestList, { ...listProps, error: 'The request list could not be loaded just now.' })
    expect(html).toContain('could not be loaded')
    expect(html).toContain('Reload list')
  })

  it('gives an owner a delete control on the selected row only, and a guest none', async () => {
    const unselected = await render(RequestList, listProps)
    expect(unselected).not.toContain('Delete request')

    const owner = await render(RequestList, { ...listProps, selectedId: '12' })
    expect(owner).toContain('aria-label="Delete request"')

    const guest = await render(RequestList, { ...listProps, selectedId: '12', owner: false })
    expect(guest).not.toContain('Delete request')
    // The guest still sees the request itself; only the mutation is absent.
    expect(guest).toContain('/orders/42')
  })
})

describe('RequestDetail', () => {
  const detail: RequestDetail = {
    ...summary,
    headers: { 'Content-Type': ['application/json'] },
    rawBody: btoa('{"a":1}'),
  }

  it('leaves deleting to the list', async () => {
    const html = await render(RequestDetailView, {
      detail, selectedId: '12', loading: false, error: '', missing: false,
    })
    expect(html).not.toContain('Delete request')
    expect(html).toContain('/orders/42')
  })

  it('says by the headers label that redacted values cannot be recovered', async () => {
    const html = await render(RequestDetailView, {
      detail, selectedId: '12', loading: false, error: '', missing: false,
    })
    expect(html).toContain('aria-label="About redacted headers"')
    expect(html).toContain('cannot be recovered')
  })

  it('explains a selection the bin no longer holds', async () => {
    const html = await render(RequestDetailView, {
      detail: null, selectedId: '12', loading: false, error: '', missing: true,
    })
    expect(html).toContain('is not in this bin')
  })

  it('prompts when nothing is selected', async () => {
    const html = await render(RequestDetailView, {
      detail: null, selectedId: null, loading: false, error: '', missing: false,
    })
    expect(html).toContain('// select a request')
  })

  it('leads with the client address when the capture came through the proxy', async () => {
    const html = await render(RequestDetailView, {
      detail: { ...detail, headers: { ...detail.headers, 'X-Forwarded-For': ['203.0.113.7'] } },
      selectedId: '12', loading: false, error: '', missing: false,
    })
    expect(textOf(html)).toContain('client ip:  203.0.113.7')
  })

  it('keeps the request on screen under a veil while the next one loads', async () => {
    const html = await render(RequestDetailView, {
      detail, selectedId: '13', loading: true, error: '', missing: false,
    })
    expect(html).toContain('class="veil"')
    expect(html).toContain('/orders/42')
    // The veil says it in the spinner, to screen readers only — no words on
    // the page, and nothing that replaces what is under it.
    expect(html).not.toContain('// Loading request')
    expect(html).toMatch(/class="sr-only"[^>]*>Loading request</)
  })

  it('veils an empty pane on the first selection, with nothing to show yet', async () => {
    const html = await render(RequestDetailView, {
      detail: null, selectedId: '12', loading: true, error: '', missing: false,
    })
    expect(html).toContain('class="veil"')
    expect(html).toContain('role="status"')
  })

  it('reports a missing or failed request rather than veiling it', async () => {
    const missing = await render(RequestDetailView, {
      detail, selectedId: '12', loading: false, error: '', missing: true,
    })
    expect(missing).not.toContain('class="veil"')
    expect(missing).not.toContain('/orders/42')

    const failed = await render(RequestDetailView, {
      detail, selectedId: '12', loading: false, error: 'This request could not be loaded just now.', missing: false,
    })
    expect(failed).not.toContain('/orders/42')
    expect(failed).toContain('Try again')
  })

  it('says nothing about a client address when the header is absent', async () => {
    const html = await render(RequestDetailView, {
      detail, selectedId: '12', loading: false, error: '', missing: false,
    })
    expect(textOf(html)).not.toContain('client ip')
  })
})

describe('SelectMenu', () => {
  it('shows the current choice and marks it among the options', async () => {
    const html = await render(SelectMenu, {
      modelValue: 'oldest',
      options: [
        { value: 'newest', label: 'newest first' },
        { value: 'oldest', label: 'oldest first' },
      ],
      label: 'Order',
    })
    expect(html).toContain('role="combobox"')
    expect(html).toContain('aria-expanded="false"')
    expect(textOf(html)).toMatch(/^\s*oldest first/)
    expect(html).toMatch(/aria-selected="true"[^>]*>\s*oldest first/)
  })
})

describe('CapacityGauge', () => {
  it('reads each limit on its own row', async () => {
    // A tenth of the request slots, four fifths of the bytes.
    const html = await render(CapacityGauge, {
      capacity: capacity({ requestCount: 50, bodyBytesUsed: 80_000_000 }),
    })
    const text = textOf(html)
    expect(text).toContain('storage')
    expect(text).toContain('requests')
    expect(text).toContain('80%')
    expect(text).toContain('10%')
    expect(html).toContain('aria-valuenow="80"')
    expect(html).toContain('aria-valuenow="10"')
    expect(html).not.toContain('alarming')
  })

  it('turns brick only on the row that is nearly out of room', async () => {
    const html = await render(CapacityGauge, { capacity: capacity({ requestCount: 460 }) })
    expect(textOf(html)).toContain('92%')
    // One of the two rows is alarming; the storage row, at 0%, is not.
    expect(html.match(/alarming/g)?.length).toBe(2)
  })

  it('reads 100% only on the limit the server says is reached', async () => {
    const html = await render(CapacityGauge, {
      capacity: capacity({ requestCount: 500, requestsFull: true, full: true }),
    })
    expect(textOf(html)).toContain('100%')
    expect(html).toContain('500 of 500 requests')
    expect(html).toContain('the limit is reached')
  })
})

describe('ServiceFull', () => {
  it('apologizes, and offers nothing the visitor cannot act on', async () => {
    const html = await render(ServiceFull, {})
    const text = textOf(html)
    expect(text).toContain('The service is at capacity')
    expect(text).toContain('try again later')
    // The frame is still there — the wordmark above, the credits below.
    expect(text).toContain('Dinu Barbu')
    expect(html).toContain('dinubarbu.com')
    // A retry would walk into the same wall. The wordmark still links home,
    // but this state offers no replacement action.
    expect(text).not.toContain('Try again')
    expect(text).not.toContain('Create New Bin')
  })
})

describe('BinUnavailable', () => {
  it.each([
    ['expired', '// this bin has expired or no longer exists'],
    ['shared', '// this shared bin no longer exists'],
  ] as const)('renders the %s explanation before the replacement action', async (kind, message) => {
    const html = await render(BinUnavailable, { kind })
    const text = textOf(html)
    expect(text).toContain(message)
    expect(text).toContain('Create New Bin')
    expect(html).toContain('href="/"')
    expect(html).toContain('aria-label="hooklook home"')
    expect(text).toContain('Dinu Barbu')
  })
})
