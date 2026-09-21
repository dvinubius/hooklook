/** The bin page's header pieces — the capture link, the help guide and its
 *  example, the clear confirmation, the switch, the info popover and the modal
 *  that holds them — rendered through Vue's server renderer like the other
 *  component tests. */

import { describe, expect, it } from 'vitest'
import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import CaptureTarget from '../components/CaptureTarget.vue'
import ClearConfirm from '../components/ClearConfirm.vue'
import ExampleRequest from '../components/ExampleRequest.vue'
import HelpGuide from '../components/HelpGuide.vue'
import InfoPopover from '../components/InfoPopover.vue'
import ModalDialog from '../components/ModalDialog.vue'
import SharePopover from '../components/SharePopover.vue'
import ToggleSwitch from '../components/ToggleSwitch.vue'

const origin = 'https://hooklook.test'

function renderModal(open: boolean): Promise<string> {
  return renderToString(
    createSSRApp({
      render: () => h(ModalDialog, { open, title: 'Bin settings' }, () => h('p', 'inside')),
    }),
  )
}

describe('ModalDialog', () => {
  it('mounts nothing inside while closed', async () => {
    const html = await renderModal(false)
    expect(html).toContain('<dialog')
    expect(html).not.toContain('inside')
    expect(html).not.toContain('Bin settings')
  })

  it('titles itself and offers a labelled close control while open', async () => {
    const html = await renderModal(true)
    expect(html).toContain('Bin settings')
    expect(html).toContain('aria-label="Close"')
    expect(html).toContain('inside')
  })
})

function render(component: Parameters<typeof h>[0], props: Record<string, unknown>): Promise<string> {
  return renderToString(createSSRApp({ render: () => h(component, props) }))
}

/** What a reader sees, with the markup taken off: highlighted phrases are
 *  wrapped in their own element, so the words are checked as text. */
function textOf(html: string): string {
  return html.replace(/<[^>]*>/g, '').replace(/\s+/g, ' ')
}

describe('HelpGuide', () => {
  const props = { captureUrl: `${origin}/b/plucky-heron-07` }

  it('explains capturing, with the example request under it', async () => {
    const html = await render(HelpGuide, props)
    expect(html).toContain('capture requests')
    expect(html).toContain('as a base URL')
    expect(html).toContain('aria-label="Copy example request"')
    expect(textOf(html)).toContain('roughly 100MB')
    expect(textOf(html)).toContain('at most 500 requests')
  })

  it('says how a bin is kept', async () => {
    const html = await render(HelpGuide, props)
    expect(html).toContain('Preserve the Bin')
    expect(textOf(html)).toContain('3 days of inactivity')
  })

  it('says credential headers are redacted and never stored', async () => {
    const html = await render(HelpGuide, props)
    expect(html).toContain('credentials in headers')
    expect(textOf(html)).toContain('replaced with [REDACTED] and never stored')
  })

  it('explains sharing without holding the guest link itself', async () => {
    const html = await render(HelpGuide, props)
    expect(html).toContain('share the bin')
    expect(textOf(html)).toContain('only valid when you enable guest access')
    expect(html).not.toContain('Copy guest link')
  })
})

describe('SharePopover', () => {
  const props = {
    enabled: true,
    saving: false,
    guestLink: `${origin}/bins/plucky-heron-07?invite=abc123`,
    origin,
  }

  it('ties its share button to a panel with the switch and the guest link', async () => {
    const html = await render(SharePopover, props)
    const target = /popovertarget="([^"]+)"/.exec(html)?.[1]
    expect(target).toBeTruthy()
    expect(html).toContain(`id="${target}"`)
    expect(html).toContain('aria-label="Share"')
    expect(html).toContain('Guest access')
    expect(html).toMatch(/role="switch" aria-checked="true"/)
    expect(html).toContain('Guest link')
    expect(html).toContain('invite=abc123')
    expect(html).toContain('aria-label="Copy guest link"')
  })

  it('holds the switch while a change is saving', async () => {
    const html = await render(SharePopover, { ...props, enabled: false, saving: true })
    expect(html).toMatch(/role="switch" aria-checked="false"[^>]*disabled/)
    expect(html).toContain('Saving access')
  })
})

describe('ClearConfirm', () => {
  it('says what emptying the bin deletes and what it keeps', async () => {
    const html = await render(ClearConfirm, { requestCount: 3, clearing: false, error: '' })
    expect(html).toContain('all 3 captured requests')
    expect(html).toContain('its guest link all stay')
    expect(html).toContain('Delete all 3')
  })

  it('shows progress, then a failure, in place', async () => {
    const clearing = await render(ClearConfirm, { requestCount: 3, clearing: true, error: '' })
    expect(clearing).toContain('Clearing…')
    const failed = await render(ClearConfirm, {
      requestCount: 3,
      clearing: false,
      error: 'hooklook could not be reached, so nothing was changed.',
    })
    expect(failed).toContain('nothing was changed')
  })
})

describe('ToggleSwitch', () => {
  it('is a labelled switch in its state, and can be held disabled', async () => {
    const on = await render(ToggleSwitch, { modelValue: true, label: 'Guest access' })
    expect(on).toContain('Guest access')
    expect(on).toMatch(/role="switch" aria-checked="true"/)
    const held = await render(ToggleSwitch, { modelValue: false, label: 'Guest access', disabled: true })
    expect(held).toMatch(/role="switch" aria-checked="false"[^>]*disabled/)
  })
})

describe('CaptureTarget', () => {
  it('shows the link, its origin receding, and no example of its own', async () => {
    const html = await renderToString(
      createSSRApp({ render: () => h(CaptureTarget, { code: 'plucky-heron-07', origin }) }),
    )
    const url = `${origin}/b/plucky-heron-07`
    // The shared origin recedes; the bin's own path follows it at full strength.
    expect(html).toMatch(new RegExp(`<span class="tok-recede"[^>]*>${origin}</span>/b/plucky-heron-07<`))
    expect(html).toContain(`title="${url}"`)
    // The example request lives in the help dialog now.
    expect(html).not.toContain('curl')
  })
})

describe('ExampleRequest', () => {
  it('shows a curl request against a path under the capture URL', async () => {
    const url = `${origin}/b/plucky-heron-07`
    const html = await renderToString(createSSRApp({ render: () => h(ExampleRequest, { url }) }))
    expect(html).toContain(`curl`)
    expect(html).toContain(`&#39;${url}&#39;`)
    expect(html).toContain(`&quot;$BASE_URL/orders/42?retry=1&quot;`)
    // Its copy control is the muted one: the accent is not spent on it.
    expect(html).toMatch(/class="copy-icon muted[^"]*"[^>]*aria-label="Copy example request"/)
  })
})

describe('InfoPopover', () => {
  it('ties its button to the note it opens', async () => {
    const html = await renderToString(
      createSSRApp({ render: () => h(InfoPopover, { label: 'About it' }, () => 'the note') }),
    )
    const target = /popovertarget="([^"]+)"/.exec(html)?.[1]
    expect(target).toBeTruthy()
    expect(html).toContain(`id="${target}"`)
    expect(html).toContain('aria-label="About it"')
    expect(html).toContain('the note')
  })
})
