<script setup lang="ts">
/* The authorized bin page: it owns the live feed, the selection, the detail
   fetch, and the owner's mutations, and it is only ever mounted once the
   session holds an authorized metadata response.

   `owner` decides what is rendered, and that is presentation only — the server
   re-checks the cookie, ownership and the request origin on every mutation, so
   a guest who calls these endpoints directly is refused there, not here. The
   invitation is never rendered except as the owner's own share link. */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import BrandMark from './BrandMark.vue'
import CaptureTarget from './CaptureTarget.vue'
import ClearConfirm from './ClearConfirm.vue'
import HelpGuide from './HelpGuide.vue'
import IconHelp from './IconHelp.vue'
import IconGithub from './IconGithub.vue'
import IconSweep from './IconSweep.vue'
import ModalDialog from './ModalDialog.vue'
import RequestDetailView from './RequestDetail.vue'
import RequestList from './RequestList.vue'
import SharePopover from './SharePopover.vue'
import ThemeToggle from './ThemeToggle.vue'
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
  detail.value = null
  detailError.value = ''
  detailMissing.value = false
  if (id === null) {
    detailLoading.value = false
    return
  }
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

const requestList = ref<InstanceType<typeof RequestList> | null>(null)

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
    // The top row of the list as shown takes over, and keeps the keyboard:
    // the delete control that had focus went with the deleted row.
    const next = requestList.value?.topId() ?? null
    select(next, true)
    if (next !== null) requestList.value?.focusRow(next)
    await feed.refetch()
  })
}

// ---- lifecycle --------------------------------------------------------

onMounted(() => {
  window.addEventListener('popstate', readSelectionFromUrl)
  props.session.onStop(() => {
    feed.stop()
    detailRequest?.abort()
  })
  feed.start()
})

onBeforeUnmount(() => {
  window.removeEventListener('popstate', readSelectionFromUrl)
  feed.stop()
  detailRequest?.abort()
})
</script>

<template>
  <div class="page">
    <header class="top">
      <BrandMark class="mark" />
      <div class="top-end">
        <ThemeToggle />
      </div>
    </header>
    <hr class="rule" />

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
          <template v-if="access.owner" #actions>
            <div class="heading-actions">
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

      <ModalDialog class="help-modal" :open="helpOpen" title="How to" @close="helpOpen = false">
        <HelpGuide :capture-url="captureUrl(access.bin.code, page.origin)" />
      </ModalDialog>

      <template v-if="access.owner">
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
      <p v-else class="meta guest">
        // read-only: this bin is shared with you, so nothing here can be changed or deleted
      </p>

      <hr class="rule" />

      <div class="workspace">
        <RequestList
          ref="requestList"
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

    <!-- As in zibs: the personal wordmark and a quiet link home, with the
         credit — pointing at the source — between them. -->
    <footer class="foot">
      <p class="wordmark">
        <span class="bracket">[ </span>Dinu Barbu<span class="bracket"> ]</span>
      </p>
      <a class="credit" href="https://github.com/dvinubius/hooklook" title="hooklook on GitHub">
        ↳ dvinubius
        <IconGithub class="github" />
        <span class="sr-only">— hooklook on GitHub</span>
      </a>
      <a class="foot-link" href="https://dinubarbu.com">→ dinubarbu.com</a>
    </footer>
  </div>
</template>

<style scoped>
/* Exactly one viewport high, so the footer is always on screen: the list and
   the detail take what is left and scroll inside it. */
.page {
  display: flex;
  flex-direction: column;
  height: 100%;
  max-width: 1180px;
  width: 100%;
  margin: 0 auto;
  padding: 16px 24px 14px;
  gap: 14px;
}
.top {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
}
.mark {
  font-size: 24px;
}
.bracket {
  color: var(--accent);
  font-weight: 700;
}
.top-end {
  display: flex;
  align-items: baseline;
  gap: 24px;
}
.body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  flex: 1;
  min-height: 0;
}
/* Sizes the capture row lays the link field and the icon buttons out with. */
.identity {
  --lead-width: 400px;
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
.guest {
  margin: 0;
}
.failure {
  margin: 0;
  padding: 12px 14px;
  background: var(--surface-shade);
  font-size: var(--text-small);
}
/* A window too short to leave this much makes the page scroll instead. */
.workspace {
  --list-width: 360px;
  --column-gap: 32px;
  position: relative;
  flex: 1;
  min-height: 280px;
  display: grid;
  grid-template-columns: var(--list-width) minmax(0, 1fr);
  grid-template-rows: minmax(0, 1fr);
  gap: var(--column-gap);
}
/* A hairline down the middle of the gap between the list and the detail. */
.workspace::before {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  left: calc(var(--list-width) + var(--column-gap) / 2);
  width: 1px;
  background: var(--hairline);
}
.foot {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: baseline;
  gap: 24px;
  padding-top: 14px;
  border-top: 1px solid var(--hairline);
}
.wordmark {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
  letter-spacing: var(--track-wordmark);
  white-space: nowrap;
}
/* Quiet link: body text over a 1px accent rule, with a leading arrow. */
/* The credit sits dead centre, whatever the two ends measure. */
.credit {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-family: var(--font-mono);
  font-size: var(--text-mono-micro);
  color: var(--text-muted);
  text-decoration: none;
}
.credit:hover {
  color: var(--text-body);
}
.github {
  width: 14px;
  height: 14px;
}
.foot-link {
  justify-self: end;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-body);
  text-decoration: none;
  border-bottom: 1px solid var(--accent);
  padding-bottom: 1px;
}
.foot-link:hover {
  color: var(--accent-on-hover);
}
</style>
