<script setup lang="ts">
/* The captured requests. The feed holds them in store order; the arrangement
   is the reader's and lives here, so a live event never reorders the list out
   from under them. Every state the list can be in is named in words — the
   brand has no animation, so a reconnecting stream says so — except a live
   stream, which is a still green dot.

   Up and down move the selection through the rows as shown. The owner's
   delete control sits on the selected row only; hiding it from a guest is
   presentation, and the server refuses a guest's delete regardless. */
import { computed, nextTick, ref } from 'vue'
import IconTrash from './IconTrash.vue'
import SelectMenu from './SelectMenu.vue'
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
  owner: boolean
  deleting: boolean
}>()

// `replace`: a move by keyboard, which replaces the history entry rather
// than adding one for every row passed on the way.
const emit = defineEmits<{ select: [id: string, replace?: boolean]; retry: []; remove: [id: string] }>()

const order = ref<SortOrder>('newest')
const filters = ref<RequestFilters>({ ...emptyFilters })

const methods = computed(() => presentMethods(props.summaries))

const orderOptions: { value: SortOrder; label: string }[] = [
  { value: 'newest', label: 'newest first' },
  { value: 'oldest', label: 'oldest first' },
]
const methodOptions = computed(() => [
  { value: '', label: 'any method' },
  ...methods.value.map((method) => ({ value: method, label: method })),
])
const narrowed = computed(() => filtersActive(filters.value))
const visible = computed(() =>
  sortSummaries(filterSummaries(props.summaries, filters.value), order.value),
)

const countNote = computed(() => {
  if (!props.loaded || props.summaries.length === 0) return ''
  if (narrowed.value && visible.value.length > 0) {
    return `${visible.value.length} of ${props.summaries.length} shown`
  }
  return `total: ${props.summaries.length}`
})

// A live stream reads "streaming" beside a green dot, like a presence status;
// every other state is named in words alone.
const streamNote = computed(() => {
  if (props.stream === 'connecting') return '// connecting…'
  if (props.stream === 'reconnecting') return '// reconnecting — the list is refreshed on request'
  return '// stream closed'
})

function clearFilters(): void {
  filters.value = { ...emptyFilters }
}

const list = ref<HTMLUListElement | null>(null)

/** The first row as the reader currently sees the list: sorted and filtered. */
function topId(): string | null {
  return visible.value[0]?.id ?? null
}

/** Puts focus on a row once it has rendered, so the arrow keys carry on from it. */
function focusRow(id: string): void {
  void nextTick(() => {
    const button = list.value?.querySelector<HTMLButtonElement>(`[data-id="${CSS.escape(id)}"]`)
    button?.focus({ preventScroll: true })
    button?.scrollIntoView({ block: 'nearest' })
  })
}

defineExpose({ topId, focusRow })

function step(event: KeyboardEvent): void {
  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
  if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return
  event.preventDefault()
  const rows = visible.value
  if (rows.length === 0) return
  const down = event.key === 'ArrowDown'
  const at = rows.findIndex((row) => row.id === props.selectedId)
  const next = at === -1 ? (down ? 0 : rows.length - 1) : at + (down ? 1 : -1)
  const target = rows[Math.max(0, Math.min(rows.length - 1, next))]
  if (target.id !== props.selectedId) emit('select', target.id, true)
  // Focus follows the selection, so the next key press continues from it.
  focusRow(target.id)
}
</script>

<template>
  <section class="list">
    <!-- Left: the label and, past a hairline, the count. Right: the stream. -->
    <header class="head">
      <h2 class="meta-caps">Requests</h2>
      <template v-if="countNote">
        <span class="separator" aria-hidden="true"></span>
        <span class="micro">{{ countNote }}</span>
      </template>
      <span
        v-if="stream === 'live'"
        class="micro stream"
        title="Live: new requests appear as they arrive"
      >
        streaming
        <span class="live" aria-hidden="true"></span>
      </span>
      <span v-else class="micro stream">{{ streamNote }}</span>
    </header>

    <div class="controls">
      <div class="control-row">
        <SelectMenu v-model="order" :options="orderOptions" label="Order" />
        <SelectMenu v-model="filters.method" :options="methodOptions" label="Filter by method" />
      </div>
      <div class="control-row">
        <input v-model="filters.path" class="field" type="search" placeholder="path contains" />
        <input v-model="filters.query" class="field" type="search" placeholder="query contains" />
      </div>
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

    <ul v-else ref="list" class="rows scroll" @keydown="step">
      <li
        v-for="item in visible"
        :key="item.id"
        class="item"
        :class="{ selected: item.id === selectedId }"
      >
        <button
          class="row"
          type="button"
          :data-id="item.id"
          :aria-current="item.id === selectedId"
          @click="emit('select', item.id)"
        >
          <span class="method">{{ item.method }}</span>
          <span class="target truncate">
            <span class="path">{{ item.path || '' }}</span>
            <span v-if="item.rawQuery" class="query">?{{ item.rawQuery }}</span>
          </span>
          <span class="when micro">{{ formatClock(item.receivedAt) }}</span>
        </button>
        <button
          v-if="owner && item.id === selectedId"
          class="remove"
          type="button"
          :aria-label="deleting ? 'Deleting request' : 'Delete request'"
          :title="deleting ? 'Deleting…' : 'Delete request'"
          :disabled="deleting"
          @click="emit('remove', item.id)"
        >
          <IconTrash class="trash" />
        </button>
      </li>
    </ul>

  </section>
</template>

<style scoped>
.list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  min-height: 0;
}
.head {
  display: flex;
  align-items: center;
  gap: 12px;
}
/* The one hue outside the brand palette: a presence-green status dot. */
.live {
  align-self: center;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #2bac76;
}
/* The facts beside the label read at 12px, a step above micro. */
.head .micro {
  font-size: var(--text-mono-meta);
}
.head h2 {
  margin: 0;
  font-size: 20px;
}
/* Selectors above, text filters below, in two matching columns. */
.controls {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.control-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
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
  /* As tall as its rows, up to what the column leaves; then it scrolls. */
  flex: 0 1 auto;
  min-height: 0;
  border-top: 1px solid var(--hairline);
}
.item {
  display: flex;
  align-items: stretch;
  transition: var(--hover-transition);
  border-bottom: 1px solid var(--hairline);
  border-left: 2px solid transparent;
}
.item:hover {
  background: var(--surface-shade);
}
/* The one accent moment in this region, and it never travels alone: the
   selected row is also the one the detail pane is showing. */
.item.selected {
  border-left-color: var(--accent);
  background: var(--surface-shade);
}
.row {
  flex: 1;
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding: 10px 12px 10px 11px;
  border: 0;
  background: transparent;
  text-align: left;
  cursor: pointer;
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  min-width: 0;
}
.remove {
  flex: none;
  display: inline-flex;
  align-items: center;
  padding: 0 12px 0 4px;
  border: 0;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
}
.remove:not(:disabled):hover {
  color: var(--text-body);
}
.remove:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.trash {
  width: 16px;
  height: 16px;
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
/* The stream state closes the header row, at its right end. */
.stream {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
</style>
