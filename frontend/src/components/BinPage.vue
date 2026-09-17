<script setup lang="ts">
/* The authorized bin page. Everything private on it comes from the metadata
   response the session already fetched — `owner` decides what an owner is told
   about their bin, and the invitation identifier is not rendered anywhere: it
   belongs to the owner's share action, which arrives with the owner controls. */
import { computed } from 'vue'
import CaptureTarget from './CaptureTarget.vue'
import { formatInstant, untilExpiry } from '../lib/format'
import { theme, toggleTheme } from '../lib/theme'
import type { BinAccess } from '../types'

const props = defineProps<{
  access: BinAccess
  origin: string
  selectedRequestId: string | null
}>()

const expiry = computed(() => untilExpiry(props.access.bin.expiresAt, Date.now()))
</script>

<template>
  <div class="page">
    <header class="top">
      <div class="mark"><span class="bracket">[</span> hooklook <span class="bracket">]</span></div>
      <div class="top-end">
        <span class="meta role">{{ access.owner ? 'your bin' : 'shared with you' }}</span>
        <button class="btn btn-quiet" type="button" @click="toggleTheme">
          // {{ theme }}
        </button>
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

      <CaptureTarget :code="access.bin.code" :origin="origin" />

      <section class="pending shade">
        <p class="meta">// captured requests appear here</p>
        <p class="pending-note">
          The live list and the request detail view are the next two steps of this build.
        </p>
        <p v-if="selectedRequestId" class="meta">
          // this URL selects request {{ selectedRequestId }}
        </p>
      </section>
    </main>

    <footer class="foot">
      <span class="wordmark"><span class="bracket">[</span> Dinu Barbu <span class="bracket">]</span></span>
      <span class="micro">↳ dvinubius</span>
    </footer>
  </div>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  min-height: 100%;
  max-width: 940px;
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
  gap: 32px;
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
.pending {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.pending-note {
  margin: 0;
  font-size: var(--text-small);
  color: var(--text-muted);
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
