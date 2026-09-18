<script setup lang="ts">
/* One request in full. The list stays body-free, so everything below the
   summary line was fetched for this selection alone. */
import { computed } from 'vue'
import BodyView from './BodyView.vue'
import HeadersView from './HeadersView.vue'
import { describeBody } from '../lib/body'
import { formatInstant } from '../lib/format'
import type { RequestDetail } from '../types'

const props = defineProps<{
  detail: RequestDetail | null
  selectedId: string | null
  loading: boolean
  error: string
  missing: boolean
  owner: boolean
  deleting: boolean
}>()

defineEmits<{ retry: []; clear: []; remove: [id: string] }>()

const body = computed(() =>
  props.detail ? describeBody(props.detail.rawBody, props.detail.contentType) : null,
)
</script>

<template>
  <section class="detail">
    <p v-if="selectedId === null" class="placeholder shade">
      <span class="meta">// no request selected</span>
      <span class="hint">Choose a request from the list to see its headers and body.</span>
    </p>

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
        <button
          v-if="owner"
          class="btn btn-danger btn-sm"
          type="button"
          :disabled="deleting"
          @click="$emit('remove', detail.id)"
        >
          {{ deleting ? 'Deleting…' : 'Delete request' }}
        </button>
      </header>

      <dl class="facts">
        <div class="fact">
          <dt class="meta">received</dt>
          <dd>{{ formatInstant(detail.receivedAt) }}</dd>
        </div>
        <div class="fact">
          <dt class="meta">content type</dt>
          <dd>{{ detail.contentType || '—' }}</dd>
        </div>
        <div class="fact">
          <dt class="meta">headers</dt>
          <dd>{{ detail.headerCount }}</dd>
        </div>
        <div class="fact">
          <dt class="meta">request id</dt>
          <dd class="mono">{{ detail.id }}</dd>
        </div>
      </dl>

      <section class="headers-block">
        <h2 class="meta-caps">Headers</h2>
        <HeadersView :headers="detail.headers" />
      </section>

      <BodyView :body="body" />
    </template>
  </section>
</template>

<style scoped>
.detail {
  display: flex;
  flex-direction: column;
  gap: 22px;
  min-width: 0;
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
.head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}
.line {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
}
.method {
  font-weight: 500;
  font-size: var(--text-title);
}
.target {
  font-size: var(--text-small);
  word-break: break-all;
}
.query {
  color: var(--text-muted);
}
.facts {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 32px;
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
  word-break: break-all;
}
.headers-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.headers-block h2 {
  margin: 0;
}
</style>
