/** The bin page's header pieces — the capture link and example, the info
 *  popover, the settings modal and the owner controls it holds — rendered
 *  through Vue's server renderer like the other component tests. */

import { describe, expect, it } from 'vitest'
import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import CaptureTarget from '../components/CaptureTarget.vue'
import InfoPopover from '../components/InfoPopover.vue'
import ModalDialog from '../components/ModalDialog.vue'
import OwnerControls from '../components/OwnerControls.vue'
import type { BinAccess } from '../types'

const origin = 'https://hooklook.test'

const access: BinAccess = {
  bin: {
    code: 'plucky-heron-07',
    createdAt: '2026-09-18T08:00:00Z',
    expiresAt: '2026-09-25T08:00:00Z',
    totalBodyBytes: 0,
  },
  owner: true,
  sharingEnabled: true,
  inviteId: 'abc123',
}

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

function renderOwner(bin: BinAccess): Promise<string> {
  return renderToString(
    createSSRApp({
      render: () => h(OwnerControls, { access: bin, origin, requestCount: 0, busy: '', error: '' }),
    }),
  )
}

describe('OwnerControls', () => {
  it('shows the guest link with a copy control inside its field', async () => {
    const html = await renderOwner(access)
    expect(html).toContain('Guest link')
    expect(html).toContain(`${origin}/bins/plucky-heron-07?invite=abc123`)
    expect(html).toContain('aria-label="Copy guest link"')
  })

  it('accents the guest link copy control only while access is shared', async () => {
    expect(await renderOwner(access)).not.toContain('copy-icon muted')
    expect(await renderOwner({ ...access, sharingEnabled: false })).toContain('copy-icon muted')
  })

  it('marks the current access mode in its toggle', async () => {
    const checked = (html: string) => /aria-checked="true"[^>]*>\s*(\w+)/.exec(html)?.[1]
    expect(checked(await renderOwner(access))).toBe('shared')
    expect(checked(await renderOwner({ ...access, sharingEnabled: false }))).toBe('private')
  })
})

describe('CaptureTarget', () => {
  it('shows the link and a curl request against a path under it', async () => {
    const html = await renderToString(
      createSSRApp({ render: () => h(CaptureTarget, { code: 'plucky-heron-07', origin }) }),
    )
    const url = `${origin}/b/plucky-heron-07`
    expect(html).toContain(`>${url}<`)
    expect(html).toContain(`curl`)
    expect(html).toContain(`&#39;${url}/orders/42?retry=1&#39;`)
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
