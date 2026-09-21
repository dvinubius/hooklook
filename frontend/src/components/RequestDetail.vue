<script setup lang="ts">
/* One request in full. The list stays body-free, so everything below the
   summary line was fetched for this selection alone. */
import { computed } from 'vue'
import BodyView from './BodyView.vue'
import HeadersTable from './HeadersTable.vue'
import InfoPopover from './InfoPopover.vue'
import { describeBody } from '../lib/body'
import { formatInstant } from '../lib/format'
import type { RequestDetail } from '../types'

const props = defineProps<{
  detail: RequestDetail | null
  selectedId: string | null
  loading: boolean
  error: string
  missing: boolean
}>()

defineEmits<{ retry: []; clear: [] }>()

const body = computed(() =>
  props.detail ? describeBody(props.detail.rawBody, props.detail.contentType) : null,
)
</script>

<template>
  <section class="detail scroll">
    <p v-if="selectedId === null" class="meta prompt">// Select a request</p>

    <p v-else-if="missing" class="placeholder shade">
      <span class="meta">// request {{ selectedId }} is not in this bin</span>
      <span class="hint">It was deleted, cleared, or the link points at another bin.</span>
      <button class="btn btn-outline btn-sm" type="button" @click="$emit('clear')">
        Back to the list
      </button>
    </p>

    <p v-else-if="error" class="placeholder shade">
      <span class="hint">{{ error }}</span>
      <button class="btn btn-outline btn-sm" type="button" @click="$emit('retry')">Try again</button>
    </p>

    <p v-else-if="loading || !detail || !body" class="placeholder shade">
      <span class="meta">// loading request {{ selectedId }}…</span>
    </p>

    <template v-else>
      <header class="head">
        <div class="line">
          <span class="method mono">{{ detail.method }}</span>
          <span class="target mono">{{ detail.path || '/' }}<span
            v-if="detail.rawQuery"
            class="query"
          >?{{ detail.rawQuery }}</span></span>
        </div>
        <p class="facts">
          <span>{{ formatInstant(detail.receivedAt) }}</span>
          <span class="separator" aria-hidden="true"></span>
          <span><span class="muted">request id: </span> {{ detail.id }}</span>
        </p>
      </header>

      <section class="headers-block">
        <div class="headers-title">
          <h2 class="meta-caps">Headers</h2>
          <InfoPopover label="About redacted headers">
            Credential headers are replaced with <strong class="redacted">[REDACTED]</strong>
            before storage. The original values were never written down and cannot be recovered
            here.
          </InfoPopover>
        </div>
        <HeadersTable :headers="detail.headers" />
      </section>

      <BodyView :body="body" />
    </template>
  </section>
</template>

<style scoped>
/* Scrolls as a whole inside its column, the headers and body with it. */
.detail {
  display: flex;
  flex-direction: column;
  gap: 22px;
  min-width: 0;
  min-height: 0;
  padding-right: 16px;
}
/* Centred in the whole pane, on the page itself — no fill. */
.prompt {
  margin: auto;
}
.placeholder {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin: 0;
  padding: 18px;
  font-size: var(--text-small);
}
.hint {
  color: var(--text-muted);
}
/* The request line, and under it its facts on one line of their own. */
.head {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.line {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
  font-size: 18px;
}
.method {
  font-weight: 500;
}
/* The path in Teal, as in the list; the query keeps its own muted grey. */
.target {
  color: var(--teal);
  word-break: break-all;
}
.query {
  color: var(--text-muted);
}
/* Received time | id, all mono, parted by the shared hairline separator. */
.facts {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 0;
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
}
.headers-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.headers-title {
  display: flex;
  align-items: center;
  gap: 8px;
}
/* The marker as the headers list shows it. */
.redacted {
  color: var(--violet);
  font-weight: 500;
}
.headers-block h2 {
  margin: 0;
  font-size: 14px;
}
</style>
