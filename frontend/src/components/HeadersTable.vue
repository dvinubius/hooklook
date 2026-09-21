<script setup lang="ts">
/* Stored headers, exactly as stored, as a table on the page itself rather
   than on a code surface, so each header is its own row between hairlines —
   the way the request list parts its rows. Names lead at full strength;
   values sit one tier under them — read, not skimmed past.

   Some values were replaced with [REDACTED] before they ever reached the
   database; they are shown as stored, in Violet — there is nothing here that could
   reconstruct them, and nothing that tries. The note saying so sits by the
   section's label, in the request detail.

   A row is shaded while hovered, and shows a copy control over the right end
   of its value that copies the whole header as `Name: value` — several
   values joined by ", ", as HTTP folds them.

   A name is cut at the column's width with an ellipsis; hovering a cut name
   shows it whole in a popover under it. The whole name is in the DOM either
   way, so a screen reader hears it without the popover. */
import { computed, onBeforeUnmount, ref } from 'vue'
import CopyButton from './CopyButton.vue'

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

    <dl v-else class="table">
      <div v-for="row in rows" :key="row.name" class="row">
        <dt class="name" @mouseenter="showName($event, row.name)" @mouseleave="hideName">{{ row.name }}</dt>
        <dd class="values">
          <span
            v-for="(entry, index) in row.values"
            :key="index"
            class="value"
            :class="{ redacted: entry.redacted }"
          >{{ entry.value }}</span>
          <CopyButton
            class="copy"
            :text="`${row.name}: ${row.values.map((entry) => entry.value).join(', ')}`"
            :label="`Copy ${row.name} header`"
            variant="icon"
            muted
          />
        </dd>
      </div>
    </dl>
    <div ref="hint" class="hint" popover="manual" aria-hidden="true">{{ hinted }}</div>
  </div>
</template>

<style scoped>
.table {
  margin: 0;
  border-top: 1px solid var(--hairline);
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  line-height: var(--leading-code);
}
/* One header per row, parted by hairlines like the request list's rows. The
   name column has a constant width; past it a name is cut with an ellipsis. */
.row {
  display: grid;
  grid-template-columns: 260px minmax(0, 1fr);
  gap: 18px;
  padding: 6px 12px;
  border-bottom: 1px solid var(--hairline);
}
.row:hover {
  background: var(--surface-shade);
}
.name {
  min-width: 0;
  color: var(--text-body);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.values {
  position: relative;
  margin: 0;
  display: flex;
  flex-direction: column;
  min-width: 0;
  color: var(--text-dim);
}
/* Level with the first line of the value, and laid over it rather than given
   room of its own: the value keeps the full width and reads whole until the
   row is hovered. Then the control appears on a solid patch of the row's
   shade, padded so the text it covers stops well short of the icon. It also
   shows while it has keyboard focus — not the focus a click leaves behind —
   and while it is saying "Copied", hovered or not. Qualified by `.values`
   so it wins over the control's own sizing. */
.values .copy {
  position: absolute;
  top: 0;
  right: -4px;
  height: calc(1em * var(--leading-code));
  min-width: 0;
  padding: 0 4px 0 4px;
  background: var(--surface-shade);
  opacity: 0;
  border: 1px solid var(--hairline);
  border-radius: var(--radius-control);
}
.row:hover .values .copy,
.values .copy:focus-visible,
.values .copy.saying {
  opacity: 1;
}
.copy :deep(.glyph) {
  width: 16px;
  height: 16px;
}
.value {
  word-break: break-all;
}
/* A value the server replaced stands out from the ones it kept. */
.value.redacted {
  color: var(--violet);
}
/* Fixed to the viewport under the name it shows; a float like the other
   popovers, so it takes their surface step and Stone edge. Long names wrap. */
.hint {
  position: fixed;
  inset: auto;
  max-width: min(480px, calc(100vw - 32px));
  margin: 0;
  padding: 6px 10px;
  border: 1px solid var(--text-muted);
  border-radius: var(--radius-surface);
  background: var(--surface-float);
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  word-break: break-all;
}
</style>
