<script setup lang="ts">
/* The authorized bin page: it owns the live feed, the selection, the detail
   fetch, and the owner's mutations, and it is only ever mounted once the
   session holds an authorized metadata response.

   `owner` decides what is rendered, and that is presentation only — the server
   re-checks the cookie, ownership and the request origin on every mutation, so
   a guest who calls these endpoints directly is refused there, not here. The
   invitation is never rendered except as the owner's own share link. */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import CaptureTarget from './CaptureTarget.vue'
import OwnerControls from './OwnerControls.vue'
import RequestDetailView from './RequestDetail.vue'
import RequestList from './RequestList.vue'
import { api, ApiError } from '../api'
import { browserFeedEnvironment, createFeed } from '../lib/feed'
import { formatInstant, untilExpiry } from '../lib/format'
import { pagePath, parseLocation } from '../lib/location'
import { theme, toggleTheme } from '../lib/theme'
import type { BinSession } from '../lib/session'
import type { BinAccess, RequestDetail } from '../types'

const props = defineProps<{ session: BinSession; access: BinAccess }>()

const page = props.session.page
const code = computed(() => props.access.bin.code)
const expiry = computed(() => untilExpiry(props.access.bin.expiresAt, Date.now()))

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

const busy = ref('')
const mutationError = ref('')

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

async function mutate(name: string, run: () => Promise<void>): Promise<void> {
  if (busy.value !== '') return
  busy.value = name
  mutationError.value = ''
  try {
    await run()
  } catch (cause) {
    mutationError.value = describeFailure(cause)
  } finally {
    busy.value = ''
  }
}

function setSharing(enabled: boolean): void {
  void mutate('sharing', async () => {
    await api.setSharing(code.value, enabled)
    await props.session.refreshAccess()
  })
}

function clearRequests(): void {
  void mutate('clear', async () => {
    await api.clearRequests(code.value)
    select(null, true)
    await feed.refetch()
  })
}

function removeRequest(id: string): void {
  void mutate('delete', async () => {
    await api.deleteRequest(code.value, id)
    feed.forget(id)
    if (selectedId.value === id) select(null, true)
    await feed.refetch()
  })
}

function replaceBin(): void {
  void mutate('replace', async () => {
    await api.replaceBin(code.value)
    // The cookie now owns a different bin. Everything here belongs to the old
    // one, so the page is torn down and `/` resolves whatever is owned now.
    feed.stop()
    props.session.stop()
    window.location.assign('/')
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
      <div class="mark"><span class="bracket">[</span> hooklook <span class="bracket">]</span></div>
      <div class="top-end">
        <span class="meta role">{{ access.owner ? 'your bin' : 'shared with you' }}</span>
        <button class="btn btn-quiet" type="button" @click="toggleTheme">// {{ theme }}</button>
      </div>
    </header>
    <hr class="rule" />

    <main class="body">
      <section class="identity">
        <h1 class="code mono">{{ access.bin.code }}</h1>
        <dl class="facts">
          <div class="fact">
            <dt class="meta">created</dt>
            <dd>{{ formatInstant(access.bin.createdAt) }}</dd>
          </div>
          <div class="fact">
            <dt class="meta">expires</dt>
            <dd>
              {{ expiry }}
              <span class="micro">· {{ formatInstant(access.bin.expiresAt) }}</span>
            </dd>
          </div>
          <div v-if="access.owner" class="fact">
            <dt class="meta">sharing</dt>
            <dd>{{ access.sharingEnabled ? 'enabled' : 'disabled' }}</dd>
          </div>
        </dl>
      </section>

      <CaptureTarget :code="access.bin.code" :origin="page.origin" />

      <OwnerControls
        v-if="access.owner"
        :access="access"
        :origin="page.origin"
        :request-count="feed.summaries.value.length"
        :busy="busy"
        :error="mutationError"
        @sharing="setSharing"
        @clear="clearRequests"
        @replace="replaceBin"
      />
      <p v-else class="meta guest">
        // read-only: this bin is shared with you, so nothing here can be changed or deleted
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
          @select="select"
          @retry="feed.refetch()"
        />
        <RequestDetailView
          :detail="detail"
          :selected-id="selectedId"
          :loading="detailLoading"
          :error="detailError"
          :missing="detailMissing"
          :owner="access.owner"
          :deleting="busy === 'delete'"
          @retry="loadDetail()"
          @clear="select(null, true)"
          @remove="removeRequest"
        />
      </div>
    </main>

    <footer class="foot">
      <span class="wordmark"
        ><span class="bracket">[</span> Dinu Barbu <span class="bracket">]</span></span
      >
      <span class="micro">↳ dvinubius</span>
    </footer>
  </div>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  min-height: 100%;
  max-width: 1180px;
  width: 100%;
  margin: 0 auto;
  padding: 28px 24px 24px;
  gap: 20px;
}
.top {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
}
.mark {
  font-size: var(--text-title);
  font-weight: 500;
  letter-spacing: var(--track-wordmark);
}
.bracket {
  color: var(--accent);
  font-weight: 700;
}
.top-end {
  display: flex;
  align-items: baseline;
  gap: 18px;
}
.body {
  display: flex;
  flex-direction: column;
  gap: 28px;
  flex: 1;
}
.identity {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.code {
  margin: 0;
  font-size: var(--text-heading);
  font-weight: 500;
  letter-spacing: var(--track-heading);
  line-height: var(--leading-heading);
  word-break: break-all;
}
.facts {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 36px;
  margin: 0;
}
.fact {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.fact dd {
  margin: 0;
  font-size: var(--text-small);
}
.guest {
  margin: 0;
}
.workspace {
  display: grid;
  grid-template-columns: minmax(300px, 400px) minmax(0, 1fr);
  gap: 32px;
  align-items: start;
}
@media (max-width: 900px) {
  .workspace {
    grid-template-columns: minmax(0, 1fr);
  }
}
.foot {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px solid var(--hairline);
}
.wordmark {
  font-size: var(--text-small);
  font-weight: 500;
  letter-spacing: var(--track-wordmark);
  color: var(--text-muted);
}
</style>
