<script setup lang="ts">
/* The authorized bin page: it owns the live feed, the selection, the detail
   fetch, and the owner's mutations, and it is only ever mounted once the
   session holds an authorized metadata response.

   `owner` decides what is rendered, and that is presentation only — the server
   re-checks the cookie, ownership and the request origin on every mutation, so
   a guest who calls these endpoints directly is refused there, not here. The
   invitation is never rendered except as the owner's own share link. */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import CapacityGauge from './CapacityGauge.vue'
import CaptureTarget from './CaptureTarget.vue'
import ClearConfirm from './ClearConfirm.vue'
import HelpGuide from './HelpGuide.vue'
import IconHelp from './IconHelp.vue'
import IconSweep from './IconSweep.vue'
import ModalDialog from './ModalDialog.vue'
import PageShell from './PageShell.vue'
import RequestDetailView from './RequestDetail.vue'
import RequestList from './RequestList.vue'
import SharePopover from './SharePopover.vue'
import { api, ApiError } from '../api'
import { browserFeedEnvironment, createFeed } from '../lib/feed'
import { captureUrl, inviteUrl, pagePath, parseLocation } from '../lib/location'
import type { BinSession } from '../lib/session'
import type { BinAccess, RequestDetail } from '../types'

const props = defineProps<{ session: BinSession; access: BinAccess }>()

const page = props.session.page
const code = computed(() => props.access.bin.code)

// ---- the live list ----------------------------------------------------

const feed = createFeed(
  browserFeedEnvironment(page.code, page.invite, props.session.signal, () => props.session.invalidate()),
)

// ---- capacity ---------------------------------------------------------
// Capacity rides on the metadata response, and SSE summaries do not carry it,
// so the page re-reads metadata whenever the list changed — a capture, a
// delete, a clear. A burst of captures is one re-read rather than one each:
// the number is a status line, not a counter, and the server is the one
// enforcing the limit either way.

const capacityRefreshDelayMs = 500
let capacityRefresh: ReturnType<typeof setTimeout> | null = null

function refreshCapacity(): void {
  if (capacityRefresh !== null) return
  capacityRefresh = setTimeout(() => {
    capacityRefresh = null
    void props.session.refreshAccess()
  }, capacityRefreshDelayMs)
}

function stopCapacityRefresh(): void {
  if (capacityRefresh === null) return
  clearTimeout(capacityRefresh)
  capacityRefresh = null
}

watch(feed.summaries, refreshCapacity)

// ---- selection, kept in the URL --------------------------------------
// Back, forward and a capture's own detail link all select the same request,
// because the URL is where the selection lives.

const selectedId = ref<string | null>(page.requestId)

function select(id: string | null, replace = false): void {
  if (selectedId.value === id && !replace) return
  selectedId.value = id
  const url = pagePath(page.code, id, page.invite)
  if (replace) window.history.replaceState({ requestId: id }, '', url)
  else window.history.pushState({ requestId: id }, '', url)
}

function readSelectionFromUrl(): void {
  selectedId.value = parseLocation(window.location.href, window.location.origin).requestId
}

// ---- the selected request's detail ------------------------------------

const detail = ref<RequestDetail | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
const detailMissing = ref(false)
let detailRequest: AbortController | null = null
let detailAttempt = 0

async function loadDetail(): Promise<void> {
  const id = selectedId.value
  // A selection that changed while the previous detail was in flight must not
  // be overwritten by the answer to a question nobody is asking any more.
  detailRequest?.abort()
  detailRequest = null
  detailAttempt += 1
  const attempt = detailAttempt
  detailError.value = ''
  detailMissing.value = false
  if (id === null) {
    detail.value = null
    detailLoading.value = false
    return
  }
  // The outgoing detail stays put while the next one is fetched; the pane
  // dims it and shows a spinner. Clearing it here is what made a selection
  // change flicker through an empty pane. An error or a missing request
  // replaces it in the pane regardless, so neither needs it cleared.
  const request = new AbortController()
  detailRequest = request
  detailLoading.value = true
  try {
    const loaded = await api.requestDetail(code.value, id, page.invite, request.signal)
    if (attempt !== detailAttempt || request.signal.aborted) return
    detail.value = loaded
  } catch (cause) {
    if (attempt !== detailAttempt || request.signal.aborted) return
    // A 404 here is about this one request, not about the visitor: the list
    // fetch and the stream are what discover revoked access.
    if (cause instanceof ApiError && cause.status === 404) detailMissing.value = true
    else detailError.value = 'This request could not be loaded just now.'
  } finally {
    if (attempt === detailAttempt) detailLoading.value = false
  }
}

watch(selectedId, () => void loadDetail(), { immediate: true })

// A selection that the list no longer contains — cleared from another tab, or
// gone with a clear-all — is reported rather than left showing stale detail.
watch([feed.summaries, feed.loaded], () => {
  const id = selectedId.value
  if (id === null || !feed.loaded.value || detailMissing.value) return
  if (feed.summaries.value.some((row) => row.id === id)) return
  detail.value = null
  detailError.value = ''
  detailMissing.value = true
})

// ---- owner mutations --------------------------------------------------

// Each kind of change is pending on its own: the server takes them
// independently, so saving access does not hold up clearing, or the reverse.
// A second click on the same kind while it is pending is ignored.
type Mutation = 'sharing' | 'clear' | 'delete'
const pending = ref(new Set<Mutation>())
const mutationError = ref('')
const clearOpen = ref(false)
const helpOpen = ref(false)

function describeFailure(cause: unknown): string {
  if (cause instanceof ApiError && cause.status === 403) {
    return 'The server refused that change. Reload the page and try again.'
  }
  if (cause instanceof ApiError && cause.status === 404) {
    return 'This bin is no longer there, so nothing was changed.'
  }
  if (cause instanceof ApiError) {
    return `The server answered ${cause.status}, so nothing was changed.`
  }
  return 'hooklook could not be reached, so nothing was changed.'
}

async function mutate(name: Mutation, run: () => Promise<void>): Promise<void> {
  if (pending.value.has(name)) return
  pending.value.add(name)
  mutationError.value = ''
  try {
    await run()
  } catch (cause) {
    mutationError.value = describeFailure(cause)
  } finally {
    pending.value.delete(name)
  }
}

function setSharing(enabled: boolean): void {
  void mutate('sharing', async () => {
    await api.setSharing(code.value, enabled)
    await props.session.refreshAccess()
  })
}

// The confirmation closes once the bin is empty; a failure keeps it open,
// with the reason, so it can be retried.
async function clearRequests(): Promise<void> {
  await mutate('clear', async () => {
    await api.clearRequests(code.value)
    select(null, true)
    await feed.refetch()
  })
  if (mutationError.value === '') clearOpen.value = false
}

const guestLink = computed(() =>
  props.access.inviteId ? inviteUrl(code.value, props.access.inviteId, page.origin) : '',
)

function removeRequest(id: string): void {
  void mutate('delete', async () => {
    await api.deleteRequest(code.value, id)
    feed.forget(id)
    // The selection is left alone: a request that is gone is reported as gone,
    // by the same watcher that catches one cleared from another tab. Moving
    // the reader to a request they did not ask for says less than the pane
    // saying what happened to the one they were reading.
    await feed.refetch()
  })
}

// ---- lifecycle --------------------------------------------------------

onMounted(() => {
  window.addEventListener('popstate', readSelectionFromUrl)
  props.session.onStop(() => {
    feed.stop()
    detailRequest?.abort()
    stopCapacityRefresh()
  })
  feed.start()
})

onBeforeUnmount(() => {
  window.removeEventListener('popstate', readSelectionFromUrl)
  feed.stop()
  detailRequest?.abort()
  stopCapacityRefresh()
})
</script>

<template>
  <PageShell>
    <main class="body">
      <section class="identity">
        <!-- A guest sees no heading; the page still names itself to a screen reader. -->
        <h1 v-if="!access.owner" class="sr-only">Shared bin</h1>
        <CaptureTarget :code="access.bin.code" :origin="page.origin">
          <template v-if="access.owner" #heading>
            <h1 class="title">Your bin</h1>
          </template>
          <template v-if="access.owner" #tools>
            <div class="heading-actions">
              <button
                class="btn btn-outline icon-button"
                type="button"
                aria-label="How to"
                title="How to"
                aria-haspopup="dialog"
                @click="helpOpen = true"
              >
                <IconHelp class="icon" />
              </button>
            </div>
          </template>
          <!-- Everything at this end acts on the bin or reports on it, and
               none of it is a guest's: they read the requests, nothing more. -->
          <template v-if="access.owner" #actions>
            <div class="heading-actions">
              <CapacityGauge :capacity="access.capacity" />
              <button
                class="btn btn-outline icon-button"
                type="button"
                aria-label="Clear all requests"
                title="Clear all requests"
                aria-haspopup="dialog"
                :disabled="feed.summaries.value.length === 0 || pending.has('clear')"
                @click="clearOpen = true"
              >
                <IconSweep class="icon" />
              </button>
              <SharePopover
                :enabled="access.sharingEnabled"
                :saving="pending.has('sharing')"
                :guest-link="guestLink"
                :origin="page.origin"
                @toggle="setSharing"
              />
            </div>
          </template>
        </CaptureTarget>
      </section>

      <template v-if="access.owner">
        <ModalDialog class="help-modal" :open="helpOpen" title="How to" @close="helpOpen = false">
          <HelpGuide :capture-url="captureUrl(access.bin.code, page.origin)" />
        </ModalDialog>

        <ModalDialog :open="clearOpen" title="Empty the bin" @close="clearOpen = false">
          <ClearConfirm
            :request-count="feed.summaries.value.length"
            :clearing="pending.has('clear')"
            :error="mutationError"
            @confirm="clearRequests"
            @cancel="clearOpen = false"
          />
        </ModalDialog>
        <!-- Emptying the bin fails inside its dialog; everything else out here. -->
        <p v-if="mutationError && !clearOpen" class="failure">{{ mutationError }}</p>
      </template>

      <!-- Global room, not this bin's: the bin below may be nearly empty and
           still take nothing, because every bin shares one store. -->
      <p v-if="access.storeCapacity.full" class="comment store-note">
        // The service is out of storage. <br/>
        // No requests will be captured until room is freed.
      </p>

      <hr class="rule" />

      <div class="workspace">
        <RequestList
          :summaries="feed.summaries.value"
          :loading="feed.loading.value"
          :loaded="feed.loaded.value"
          :stream="feed.stream.value"
          :error="feed.error.value"
          :selected-id="selectedId"
          :owner="access.owner"
          :deleting="pending.has('delete')"
          @select="select"
          @retry="feed.refetch()"
          @remove="removeRequest"
        />
        <RequestDetailView
          :detail="detail"
          :selected-id="selectedId"
          :loading="detailLoading"
          :error="detailError"
          :missing="detailMissing"
          @retry="loadDetail()"
          @clear="select(null, true)"
        />
      </div>
    </main>
  </PageShell>
</template>

<style scoped>
/* The shell is one viewport high and the bars take their own height; this is
   what is left, and the list and the detail scroll inside it. */
.body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  flex: 1;
  min-height: 0;
}
/* Sizes the capture row lays the link field and the icon buttons out with. */
.identity {
  --row-height: 36px;
}
.title {
  flex: none;
  margin: 0;
  font-size: 20px;
  font-weight: 500;
  letter-spacing: var(--track-heading);
  line-height: var(--leading-heading);
}
/* The help dialog stays at most 680px tall and scrolls inside past that. */
/* Taller than this and the guide scrolls inside its own dialog. */
.help-modal {
  max-height: min(680px, calc(100vh - 32px));
}
.heading-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
/* Square icon buttons, as high as the link field below them. */
.icon-button {
  flex: none;
  width: var(--row-height);
  height: var(--row-height);
  padding: 0;
  justify-content: center;
}
.icon {
  width: 20px;
  height: 20px;
}
/* The one line on the page that is neither this visitor's doing nor fixable
   by them, and it decides whether anything more arrives at all — so it takes
   the brand's danger colour rather than the quiet register of the notes
   around it. */
.store-note {
  margin: 0;
  color: var(--danger);
}
.failure {
  margin: 0;
  padding: 12px 14px;
  background: var(--surface-shade);
  font-size: var(--text-small);
}
/* A window too short to leave this much makes the page scroll instead. The
   two panes are parted by the list's own fill now, not by a hairline down
   the gutter. */
.workspace {
  --list-width: 440px;
  --column-gap: 32px;
  flex: 1;
  min-height: 280px;
  display: grid;
  grid-template-columns: var(--list-width) minmax(0, 1fr);
  grid-template-rows: minmax(0, 1fr);
  gap: var(--column-gap);
}
</style>
