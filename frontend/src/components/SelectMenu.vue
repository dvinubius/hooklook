<script setup lang="ts" generic="T extends string">
/* A single-choice dropdown in the brand's own terms, in place of a native
   `<select>`: a square field with the choice and a chevron, opening a flat
   list of options below it. It follows the select-only combobox pattern —
   focus stays on the field, the options are announced as active descendants.

   Keys: ↓ ↑ Enter or Space open it; while open ↓ ↑ Home End move, Enter or
   Space choose, Esc and Tab close. A press anywhere outside closes it. */
import { computed, onBeforeUnmount, ref, useId, watch } from 'vue'

const model = defineModel<T>({ required: true })
const props = defineProps<{ options: { value: T; label: string }[]; label: string }>()

const id = useId()
const root = ref<HTMLElement | null>(null)
const open = ref(false)
const active = ref(0)

const selectedIndex = computed(() =>
  Math.max(0, props.options.findIndex((option) => option.value === model.value)),
)
const current = computed(() => props.options[selectedIndex.value]?.label ?? '')
const optionId = (index: number) => `${id}-option-${index}`

function show(): void {
  active.value = selectedIndex.value
  open.value = true
}

function hide(): void {
  open.value = false
}

function choose(index: number): void {
  const option = props.options[index]
  if (option) model.value = option.value
  hide()
}

function key(event: KeyboardEvent): void {
  const last = props.options.length - 1
  if (!open.value) {
    if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(event.key)) {
      event.preventDefault()
      show()
    }
    return
  }
  switch (event.key) {
    case 'ArrowDown':
      active.value = Math.min(last, active.value + 1)
      break
    case 'ArrowUp':
      active.value = Math.max(0, active.value - 1)
      break
    case 'Home':
      active.value = 0
      break
    case 'End':
      active.value = last
      break
    case 'Enter':
    case ' ':
      choose(active.value)
      break
    case 'Escape':
      hide()
      break
    case 'Tab':
      hide()
      return
    default:
      return
  }
  event.preventDefault()
}

function outside(event: PointerEvent): void {
  if (!root.value?.contains(event.target as Node)) hide()
}

watch(open, (isOpen) => {
  if (isOpen) document.addEventListener('pointerdown', outside, true)
  else document.removeEventListener('pointerdown', outside, true)
})

onBeforeUnmount(() => document.removeEventListener('pointerdown', outside, true))
</script>

<template>
  <div ref="root" class="select">
    <button
      class="trigger"
      type="button"
      role="combobox"
      :aria-label="label"
      aria-haspopup="listbox"
      :aria-expanded="open"
      :aria-controls="`${id}-list`"
      :aria-activedescendant="open ? optionId(active) : undefined"
      @click="open ? hide() : show()"
      @keydown="key"
    >
      <span class="current truncate">{{ current }}</span>
      <svg class="chevron" :class="{ up: open }" viewBox="0 0 10 6" fill="none"
        stroke="currentColor" stroke-width="1.3" aria-hidden="true" focusable="false">
        <path d="M0.5 0.5L5 5L9.5 0.5" />
      </svg>
    </button>
    <ul v-show="open" :id="`${id}-list`" class="menu" role="listbox" :aria-label="label">
      <li
        v-for="(option, index) in options"
        :id="optionId(index)"
        :key="option.value"
        class="option"
        :class="{ active: index === active, chosen: index === selectedIndex }"
        role="option"
        :aria-selected="index === selectedIndex"
        @pointerenter="active = index"
        @click="choose(index)"
      >
        {{ option.label }}
      </li>
    </ul>
  </div>
</template>

<style scoped>
.select {
  position: relative;
  min-width: 0;
}
/* The same box as the text fields beside it: mono, hairline, page fill. */
.trigger {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 7px 12px 7px 9px;
  border: 1px solid var(--hairline);
  border-radius: var(--radius-control);
  background: var(--surface-page);
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  text-align: left;
  cursor: pointer;
}
.trigger:hover,
.trigger[aria-expanded='true'] {
  border-color: var(--text-muted);
}
.current {
  flex: 1;
  min-width: 0;
  color: var(--text-dim);
}
.chevron {
  flex: none;
  width: 10px;
  height: 6px;
  color: var(--text-muted);
}
.chevron.up {
  transform: rotate(180deg);
}
/* Flat, softly cornered, bordered — no shadow; it sits over the rows below
   it, and clips its options so they keep to its corners. The Stone edge here
   is what the other floats were given too; on dark the menu used to be
   darker than the shaded rows it covered. */
.menu {
  position: absolute;
  top: calc(100% - 1px);
  left: 0;
  right: 0;
  z-index: 10;
  max-height: 240px;
  overflow: hidden auto;
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--text-muted);
  border-radius: var(--radius-surface);
  background: var(--surface-float);
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
}
.option {
  padding: 7px 9px;
  color: var(--text-dim);
  cursor: pointer;
}
.option.chosen {
  color: var(--text-body);
}
/* A step off the menu's own fill, not off the page's: `--surface-shade` is
   below the float on dark, so the highlighted option would have read as a
   hole in the menu. Mixing from the float moves toward Paper on dark and
   toward Ink on light — away from the surface either way. */
.option.active {
  background: color-mix(in srgb, var(--surface-float) 92%, var(--text-body));
  color: var(--text-body);
}
</style>
