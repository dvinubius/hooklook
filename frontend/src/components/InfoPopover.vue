<script setup lang="ts">
/* A small "i" that opens a note on click, on the native Popover API: the
   browser toggles it from the button, and closes it on Esc or a click
   elsewhere. The note is placed under the button just before it opens; it
   has a fixed width so nothing has to be measured first. */
import { ref, useId } from 'vue'

defineProps<{ label: string }>()

const id = useId()
const trigger = ref<HTMLButtonElement | null>(null)
const note = ref<HTMLElement | null>(null)

const width = 280
const gutter = 16

function place(event: Event): void {
  if ((event as ToggleEvent).newState !== 'open') return
  const button = trigger.value?.getBoundingClientRect()
  const element = note.value
  if (!button || !element) return
  const left = Math.max(gutter, Math.min(button.left, window.innerWidth - width - gutter))
  // Document coordinates, so the note scrolls with the button it belongs to.
  element.style.top = `${button.bottom + window.scrollY + 6}px`
  element.style.left = `${left + window.scrollX}px`
}
</script>

<template>
  <button ref="trigger" class="info" type="button" :aria-label="label" :popovertarget="id">i</button>
  <div :id="id" ref="note" class="note" popover @beforetoggle="place">
    <slot />
  </div>
</template>

<style scoped>
.info {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  padding: 0;
  border: 1px solid var(--hairline);
  background: transparent;
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: var(--text-mono-micro);
  line-height: 1;
  cursor: pointer;
  vertical-align: middle;
}
.info:hover {
  border-color: var(--text-body);
  color: var(--text-body);
}
.note {
  position: absolute;
  inset: auto;
  width: 280px; /* = width in the script */
  max-width: calc(100vw - 32px);
  margin: 0;
  padding: 12px 14px;
  border: 1px solid var(--hairline);
  background: var(--surface-page);
  color: var(--text-body);
  font-size: var(--text-small);
  line-height: var(--leading-small);
  /* It lives inside a no-wrap fact in the DOM, and popovers scroll by default:
     the note wraps instead, and grows to fit what it says. */
  white-space: normal;
  overflow: visible;
}
</style>
