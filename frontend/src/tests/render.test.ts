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
    expect(html).toContain('this request had no body')
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
    expect(html).toContain('cannot be recovered')
    expect(html).toContain('application/json')
  })

  it('escapes a header value as readily as a body', async () => {
    const html = await render(HeadersView, { headers: { 'X-Note': ['<b>hi</b>'] } })
    expect(html).not.toMatch(/<b>hi<\/b>/)
    expect(textOf(html)).toContain('<b>hi</b>')
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
    expect(html).toContain('1 captured')
  })

  it('distinguishes nothing-yet from nothing-matching', async () => {
    const empty = await render(RequestList, { ...listProps, summaries: [] })
    expect(empty).toContain('nothing captured yet')

    const loading = await render(RequestList, { ...listProps, loading: true, loaded: false, summaries: [] })
    expect(loading).toContain('loading captured requests')
  })

  it('names the stream state in words rather than animating it', async () => {
    const live = await render(RequestList, listProps)
    expect(live).toContain('// live')
    const down = await render(RequestList, { ...listProps, stream: 'reconnecting' })
    expect(down).toContain('reconnecting')
  })

  it('offers a reload when the list itself could not be read', async () => {
    const html = await render(RequestList, { ...listProps, error: 'The request list could not be loaded just now.' })
    expect(html).toContain('could not be loaded')
    expect(html).toContain('Reload list')
  })
})

describe('RequestDetail', () => {
  const detail: RequestDetail = {
    ...summary,
    headers: { 'Content-Type': ['application/json'] },
    rawBody: btoa('{"a":1}'),
  }

  it('gives an owner a delete control and a guest none', async () => {
    const owner = await render(RequestDetailView, {
      detail, selectedId: '12', loading: false, error: '', missing: false, owner: true, deleting: false,
    })
    expect(owner).toContain('Delete request')

    const guest = await render(RequestDetailView, {
      detail, selectedId: '12', loading: false, error: '', missing: false, owner: false, deleting: false,
    })
    expect(guest).not.toContain('Delete request')
    // The guest still sees the request itself; only the mutation is absent.
    expect(guest).toContain('/orders/42')
  })

  it('explains a selection the bin no longer holds', async () => {
    const html = await render(RequestDetailView, {
      detail: null, selectedId: '12', loading: false, error: '', missing: true, owner: true, deleting: false,
    })
    expect(html).toContain('is not in this bin')
    expect(html).toContain('Back to the list')
  })

  it('prompts when nothing is selected', async () => {
    const html = await render(RequestDetailView, {
      detail: null, selectedId: null, loading: false, error: '', missing: false, owner: true, deleting: false,
    })
    expect(html).toContain('no request selected')
  })
})
