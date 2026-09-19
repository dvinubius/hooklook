/** What the components actually put on the page.
 *
 * Rendered in Node through Vue's own server renderer, so these assert real
 * output — above all that a captured body reaches the page as characters and
 * never as markup, and that a guest is shown no control they cannot use. */

import { describe, expect, it } from 'vitest'
import { createSSRApp, h, type Component } from 'vue'
import { renderToString } from 'vue/server-renderer'
import BodyView from '../components/BodyView.vue'
import HeadersView from '../components/HeadersView.vue'
import RequestDetailView from '../components/RequestDetail.vue'
import RequestList from '../components/RequestList.vue'
import SelectMenu from '../components/SelectMenu.vue'
import { describeBody } from '../lib/body'
import type { RequestDetail, RequestSummary } from '../types'

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

describe('HeadersView', () => {
  it('shows a redacted value as stored and does not pretend to recover it', async () => {
    const html = await render(HeadersView, {
      headers: { Authorization: ['[REDACTED]'], 'Content-Type': ['application/json'] },
    })
    expect(html).toContain('[REDACTED]')
    expect(html).toContain('application/json')
  })

  it('escapes a header value as readily as a body', async () => {
    const html = await render(HeadersView, { headers: { 'X-Note': ['<b>hi</b>'] } })
    expect(html).not.toMatch(/<b>hi<\/b>/)
    expect(textOf(html)).toContain('<b>hi</b>')
  })

  it('keeps a long name whole in the page, however it is cut on screen', async () => {
    const name = 'X-Very-Long-Vendor-Specific-Signature-Header'
    const html = await render(HeadersView, { headers: { [name]: ['v'] } })
    expect(html).toMatch(new RegExp(`<dt class="name"[^>]*>${name}</dt>`))
  })

  it('says so when nothing was stored', async () => {
    const html = await render(HeadersView, { headers: {} })
    expect(html).toContain('no headers were stored')
  })
})

describe('RequestList', () => {
  it('shows each capture with its method, path and query', async () => {
    const html = await render(RequestList, listProps)
    expect(html).toContain('POST')
    expect(html).toContain('/orders/42')
    expect(html).toContain('?retry=1')
    expect(html).toContain('Total requests: 1')
  })

  it('distinguishes nothing-yet from nothing-matching', async () => {
    const empty = await render(RequestList, { ...listProps, summaries: [] })
    expect(empty).toContain('nothing captured yet')

    const loading = await render(RequestList, { ...listProps, loading: true, loaded: false, summaries: [] })
    expect(loading).toContain('loading captured requests')
  })

  it('shows a live stream as streaming with a status dot, and anything else as connecting', async () => {
    const live = await render(RequestList, listProps)
    expect(live).toContain('class="live"')
    expect(textOf(live)).toContain('streaming')
    for (const stream of ['connecting', 'reconnecting', 'closed'] as const) {
      const down = await render(RequestList, { ...listProps, stream })
      expect(textOf(down)).toContain('connecting…')
      expect(down).not.toContain('class="live"')
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
    expect(html).toContain('Back to the list')
  })

  it('prompts when nothing is selected', async () => {
    const html = await render(RequestDetailView, {
      detail: null, selectedId: null, loading: false, error: '', missing: false,
    })
    expect(html).toContain('// Select a request')
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
