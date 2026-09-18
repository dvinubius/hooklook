<script setup lang="ts">
/* The captured requests. The feed holds them in store order; the arrangement
   is the reader's and lives here, so a live event never reorders the list out
   from under them. Every state the list can be in is named in words — the
   brand has no animation, so a reconnecting stream says so. */
import { computed, ref } from 'vue'
import { formatClock } from '../lib/format'
import { filterSummaries, filtersActive, presentMethods, sortSummaries } from '../lib/list'
import {
  emptyFilters,
  type RequestFilters,
  type RequestSummary,
  type SortOrder,
  type StreamState,
} from '../types'

const props = defineProps<{
  summaries: RequestSummary[]
  loading: boolean
  loaded: boolean
  stream: StreamState
  error: string
  selectedId: string | null
}>()

defineEmits<{ select: [id: string]; retry: [] }>()

const order = ref<SortOrder>('newest')
const filters = ref<RequestFilters>({ ...emptyFilters })

const methods = computed(() => presentMethods(props.summaries))
const narrowed = computed(() => filtersActive(filters.value))
const visible = computed(() =>
  sortSummaries(filterSummaries(props.summaries, filters.value), order.value),
)

const streamNote = computed(() => {
  if (props.stream === 'live') return '// live'
  if (props.stream === 'connecting') return '// connecting…'
  if (props.stream === 'reconnecting') return '// reconnecting — the list is refreshed on request'
  return '// stream closed'
})

function clearFilters(): void {
  filters.value = { ...emptyFilters }
}
</script>

<template>
  <section class="list">
    <header class="head">
      <h2 class="meta-caps">Requests</h2>
      <span class="micro">{{ streamNote }}</span>
    </header>

    <div class="controls">
      <select v-model="order" class="field" aria-label="Order">
        <option value="newest">newest first</option>
        <option value="oldest">oldest first</option>
      </select>
      <select v-model="filters.method" class="field" aria-label="Filter by method">
        <option value="">any method</option>
        <option v-for="method in methods" :key="method" :value="method">{{ method }}</option>
      </select>
      <input v-model="filters.path" class="field grow" type="search" placeholder="path contains" />
      <input v-model="filters.query" class="field grow" type="search" placeholder="query contains" />
    </div>

    <p v-if="error" class="state">
      {{ error }}
      <button class="btn btn-outline btn-sm" type="button" @click="$emit('retry')">Reload list</button>
    </p>

    <p v-if="loading && !loaded" class="meta state">// loading captured requests…</p>

    <p v-else-if="summaries.length === 0" class="state">
      <span class="meta">// nothing captured yet</span>
      <span class="hint">Send anything to the capture URL above and it appears here without a reload.</span>
    </p>

    <p v-else-if="visible.length === 0" class="state">
      <span class="meta">// no request matches these filters</span>
      <span class="hint">{{ summaries.length }} captured in total.</span>
      <button class="btn btn-outline btn-sm" type="button" @click="clearFilters">Clear filters</button>
    </p>

    <ul v-else class="rows scroll">
      <li v-for="item in visible" :key="item.id">
        <button
          class="row"
          :class="{ selected: item.id === selectedId }"
          type="button"
          :aria-current="item.id === selectedId"
          @click="$emit('select', item.id)"
        >
          <span class="method">{{ item.method }}</span>
          <span class="target truncate">
            <span class="path">{{ item.path || '/' }}</span>
            <span v-if="item.rawQuery" class="query">?{{ item.rawQuery }}</span>
          </span>
          <span class="when micro">{{ formatClock(item.receivedAt) }}</span>
        </button>
      </li>
    </ul>

    <p v-if="loaded && narrowed && visible.length > 0" class="micro count">
      {{ visible.length }} of {{ summaries.length }} shown
    </p>
    <p v-else-if="loaded && summaries.length > 0" class="micro count">
      {{ summaries.length }} captured
    </p>
  </section>
</template>

<style scoped>
.list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}
.head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}
.head h2 {
  margin: 0;
}
.controls {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.grow {
  flex: 1 1 130px;
  min-width: 0;
}
.state {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin: 0;
  padding: 14px;
  background: var(--surface-shade);
  font-size: var(--text-small);
}
.hint {
  color: var(--text-muted);
  font-size: var(--text-small);
}
.rows {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 60vh;
  border-top: 1px solid var(--hairline);
}
.rows li {
  border-bottom: 1px solid var(--hairline);
}
.row {
  display: flex;
  align-items: baseline;
  gap: 12px;
  width: 100%;
  padding: 10px 12px 10px 11px;
  border: 0;
  border-left: 2px solid transparent;
  background: transparent;
  text-align: left;
  cursor: pointer;
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  min-width: 0;
}
.row:hover {
  background: var(--surface-shade);
}
/* The one accent moment in this region, and it never travels alone: the
   selected row is also the one the detail pane is showing. */
.row.selected {
  border-left-color: var(--accent);
  background: var(--surface-shade);
}
.method {
  font-weight: 500;
  color: var(--text-body);
  min-width: 4.5em;
}
.target {
  flex: 1;
  min-width: 0;
}
.path {
  color: var(--text-body);
}
.query {
  color: var(--text-muted);
}
.when {
  flex: none;
}
.count {
  margin: 0;
}
</style>
