<script setup lang="ts">
/* Stored headers, exactly as stored. Some values were replaced before they
   ever reached the database; those are marked as redacted and left that way —
   there is nothing here that could reconstruct them, and nothing that tries.
   The note saying so sits by the section's label, in the request detail.

   A name is cut to 260px with an ellipsis; hovering a cut name shows it whole
   in a popover under it. The whole name is in the DOM either way, so a screen
   reader hears it without the popover. */
import { computed, onBeforeUnmount, ref } from 'vue'

const props = defineProps<{ headers: Record<string, string[]> }>()

const redactedMarker = '[REDACTED]'

const rows = computed(() =>
  Object.entries(props.headers)
    .sort(([left], [right]) => (left.toLowerCase() < right.toLowerCase() ? -1 : 1))
    .map(([name, values]) => ({
      name,
      values: values.map((value) => ({ value, redacted: value === redactedMarker })),
    })),
)

// One popover serves every name; it is placed once, so a scroll anywhere
// would leave it behind, and it closes instead.
const hint = ref<HTMLElement | null>(null)
const hinted = ref('')

function showName(event: MouseEvent, name: string): void {
  const cell = event.currentTarget as HTMLElement
  const element = hint.value
  if (!element || cell.scrollWidth <= cell.clientWidth) return
  hinted.value = name
  const box = cell.getBoundingClientRect()
  element.style.top = `${box.bottom + 4}px`
  element.style.left = `${box.left}px`
  element.showPopover()
  window.addEventListener('scroll', hideName, true)
}

function hideName(): void {
  window.removeEventListener('scroll', hideName, true)
  if (hint.value?.matches(':popover-open')) hint.value.hidePopover()
}

onBeforeUnmount(hideName)
</script>

<template>
  <div class="headers">
    <p v-if="rows.length === 0" class="meta">// no headers were stored</p>

    <dl v-else class="table code-surface scroll">
      <template v-for="row in rows" :key="row.name">
        <dt class="name" @mouseenter="showName($event, row.name)" @mouseleave="hideName">{{ row.name }}</dt>
        <dd class="values">
          <span
            v-for="(entry, index) in row.values"
            :key="index"
            class="value"
            :class="entry.redacted ? 'tok-recede' : 'tok-body'"
          >{{ entry.value }}</span>
        </dd>
      </template>
    </dl>
    <div ref="hint" class="hint" popover="manual" aria-hidden="true">{{ hinted }}</div>
  </div>
</template>

<style scoped>
.headers {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.table {
  display: grid;
  grid-template-columns: minmax(8rem, max-content) 1fr;
  gap: 2px 18px;
  margin: 0;
  padding: 14px 16px;
  max-height: 700px;
  font-size: var(--text-mono-meta);
}
/* The column has constant width: 260px; past that a name
   is cut with an ellipsis. */
.name {
  width: 260px;
  color: var(--paper);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
/* Fixed to the viewport under the name it shows; flat and hairline-bordered,
   like the other popovers. Long names wrap. */
.hint {
  position: fixed;
  inset: auto;
  max-width: min(480px, calc(100vw - 32px));
  margin: 0;
  padding: 6px 10px;
  border: 1px solid var(--hairline);
  background: var(--surface-page);
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  word-break: break-all;
}
.values {
  margin: 0;
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.value {
  word-break: break-all;
}
</style>
