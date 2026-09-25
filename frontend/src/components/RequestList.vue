<script setup lang="ts">
/* The captured requests. The feed holds them in store order; the arrangement
   is the reader's and lives here, so a live event never reorders the list out
   from under them. Every state the list can be in is named in words — the
   brand has no animation. The stream reads "streaming" beside a still green
   dot while it is live, and "connecting…" beside a grey one otherwise: a
   first connection and a reconnection look the same to the reader. How full
   the bin is is not said here at all — that is the capture row's gauge, a
   fact about the bin rather than about this list.

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
  type TextOperator,
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
const filters = ref<RequestFilters>(emptyFilters())

const methods = computed(() => presentMethods(props.summaries))

const orderOptions: { value: SortOrder; label: string }[] = [
  { value: 'newest', label: 'new first' },
  { value: 'oldest', label: 'old first' },
]
const operatorOptions: { value: TextOperator; label: string }[] = [
  { value: 'matches', label: 'matches' },
  { value: 'empty', label: 'is empty' },
  { value: 'notEmpty', label: 'is not empty' },
]
const methodOptions = computed(() => [
  { value: '', label: 'any' },
  ...methods.value.map((method) => ({ value: method, label: method })),
])
const narrowed = computed(() => filtersActive(filters.value))
const visible = computed(() =>
  sortSummaries(filterSummaries(props.summaries, filters.value), order.value),
)

// Only counted while a filter is on, and only once there is a list to count.
// An unfiltered list needs no number: what is on screen is what there is.
const showingCounts = computed(() =>
  props.loaded && props.summaries.length > 0 && narrowed.value
    ? { shown: visible.value.length, total: props.summaries.length }
    : null,
)

function clearFilters(): void {
  filters.value = emptyFilters()
}

const list = ref<HTMLUListElement | null>(null)

/** Puts focus on a row once it has rendered, so the arrow keys carry on from it. */
function focusRow(id: string): void {
  void nextTick(() => {
    const button = list.value?.querySelector<HTMLButtonElement>(`[data-id="${CSS.escape(id)}"]`)
    button?.focus({ preventScroll: true })
    button?.scrollIntoView({ block: 'nearest' })
  })
}

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
    <!-- The section label with the stream state at its right end. -->
    <header class="head">
      <h2 class="caps">Requests</h2>
      <span
        v-if="stream === 'live'"
        class="fact stream"
        title="Live: new requests appear as they arrive"
      >
        streaming
        <span class="dot live" aria-hidden="true"></span>
      </span>
      <span v-else class="fact stream">
        connecting…
        <span class="dot waiting" aria-hidden="true"></span>
      </span>
    </header>

    <!-- One row per control, each named at its left: the text filters — an
         operator, and the text a "matches" row adds — then method and sort. -->
    <div class="controls">
      <div class="control-row">
        <span class="fact" aria-hidden="true">path</span>
        <SelectMenu v-model="filters.path.operator" :options="operatorOptions" label="Path filter" />
        <input
          v-if="filters.path.operator === 'matches'"
          v-model="filters.path.text"
          class="field"
          type="search"
          placeholder="text"
          aria-label="Path text"
        />
      </div>
      <div class="control-row">
        <span class="fact" aria-hidden="true">query</span>
        <SelectMenu v-model="filters.query.operator" :options="operatorOptions" label="Query filter" />
        <input
          v-if="filters.query.operator === 'matches'"
          v-model="filters.query.text"
          class="field"
          type="search"
          placeholder="text"
          aria-label="Query text"
        />
      </div>
      <!-- The last row carries the two whole-list controls: what it keeps at
           the left, the arrangement of what is left at the right. -->
      <div class="control-row pair">
        <span class="fact" aria-hidden="true">method</span>
        <div class="pair-left">
          <SelectMenu
            v-model="filters.method"
            class="method"
            :options="methodOptions"
            label="Filter by method"
          />
          <span class="fact" aria-hidden="true">sort</span>
        </div>
        <SelectMenu v-model="order" :options="orderOptions" label="Order" />
      </div>
    </div>

    <!-- What the filters left, between lines of its own: it belongs to the
         controls above it and to the rows below it equally, and stands apart
         from both. -->
    <template v-if="showingCounts">
      <div class="divider" aria-hidden="true"></div>
      <p class="showing">
        <!-- The number that changes with the filter is the one being read;
             the total it is out of stays a tier back. -->
        <span class="fact">Results: <span class="shown">{{ showingCounts.shown }}</span> (Total:
          {{ showingCounts.total }})</span>
      </p>
    </template>

    <p v-if="error" class="state">
      {{ error }}
      <button class="btn btn-outline btn-sm" type="button" @click="$emit('retry')">Reload list</button>
    </p>

    <p v-if="loading && !loaded" class="comment state">// loading captured requests…</p>

    <p v-else-if="summaries.length === 0" class="state empty">
      <span class="comment">// no requests captured yet</span>
    </p>

    <!-- What a filter leaves stands where the rows would, under the same
         line — whether that is rows or nothing. -->
    <template v-else-if="visible.length === 0">
      <div class="divider" aria-hidden="true"></div>
      <p class="state">
        <span class="comment">// no request matches</span>
        <button class="btn btn-outline btn-sm clear-filter-btn" type="button" @click="clearFilters">Clear filters</button>
      </p>
    </template>

    <template v-else>
      <div class="divider" aria-hidden="true"></div>
      <ul ref="list" class="rows scroll" @keydown="step">
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
            <span class="target truncate">{{ item.path || '' }}<span
              v-if="item.rawQuery"
              class="query"
            >?{{ item.rawQuery }}</span></span>
            <span class="when micro muted">{{ formatClock(item.receivedAt) }}</span>
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
    </template>
  </section>
</template>

<style scoped>
/* The master column is a filled region, which is what parts it from the
   detail — the workspace hairline that used to do that alone is gone. Rung 1
   of the shade ramp; everything inside steps off this rather than off the
   page, and the hairlines parting the rows take the value tuned to it. */
.list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  min-height: 0;
  padding: 12px;
  background: var(--surface-shade);
  border-radius: var(--radius-surface);
  --hairline: var(--shade-hairline);
}
.head {
  display: flex;
  align-items: center;
  gap: 12px;
}
.head h2 {
  margin: 0;
  font-size: 16px;
}
/* The status dot: one shape, two readings. Green is the one hue outside the
   brand palette; grey says the same thing the word beside it does. */
.dot {
  align-self: center;
  width: 10px;
  height: 10px;
  border-radius: 50%;
}
.live {
  background: #2bac76;
}
.waiting {
  background: var(--text-muted);
}
.showing {
  margin: 0;
}
.showing .shown {
  color: var(--text-body);
}
/* One row per text filter, on a shared grid: its name, then its operator in
   a column of one width, then the text a "matches" row adds. */
.controls {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.control-row {
  display: grid;
  grid-template-columns: 3.5em minmax(0, 1fr) minmax(0, 1fr);
  align-items: center;
  gap: 8px;
}
/* Method and sort share the last row on the same three columns as the text
   filters: the method choice and the "sort" label together in the operator
   column, the arrangement itself in the text column — squared up with the
   fields above it. The choice takes what the label beside it leaves. */
/* Room alone parts what the list keeps from how it is arranged — several
   times the 8px that binds "sort" to its own control, so each label still
   reads with the control it names, and no line is needed to say so. */
.pair-left {
  display: flex;
  align-items: center;
  gap: 28px;
  min-width: 0;
}
.pair-left .method {
  flex: 1;
  min-width: 0;
}
.state {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin: 0;
  padding: 14px;
  background: var(--surface-shade-2);
  border-radius: var(--radius-surface);
  font-size: var(--text-small);
}
/* An empty bin has nothing below the controls, so its note sits in the
   middle of the room the column leaves, as the detail prompt does — on the
   column itself, without a surface of its own. */
.empty {
  margin-block: auto;
  justify-content: center;
  background: none;
}
.clear-filter-btn {
  margin-left: auto;
}
/* The line is its own element, so what it introduces starts clear of it. */
.divider {
  height: 1px;
  background: var(--hairline);
}
.rows {
  list-style: none;
  margin: 0;
  padding: 0;
  /* As tall as its rows, up to what the column leaves; then it scrolls. */
  flex: 0 1 auto;
  min-height: 0;
}
.item {
  display: flex;
  align-items: stretch;
  border-bottom: 1px solid var(--hairline);
  border-left: 2px solid transparent;
}
.item:hover {
  background: var(--surface-shade-2);
}
/* The one accent moment in this region, and it never travels alone: the
   selected row is also the one the detail pane is showing. */
.item.selected {
  border-left-color: var(--accent);
  background: var(--surface-shade-2);
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
  opacity: var(--disabled-opacity);
  cursor: not-allowed;
}
.trash {
  width: 16px;
  height: 16px;
}
.method {
  font-weight: 400;
  color: var(--text-body);
  min-width: 4.5em;
}
.target {
  flex: 1;
  min-width: 0;
}
/* The path reads as the row's body text; the raw query recedes behind it. */
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
