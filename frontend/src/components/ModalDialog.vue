<script setup lang="ts">
/* A modal on the native `<dialog>`: the browser supplies the top layer, the
   focus trap and Esc. The parent owns `open`; every way of closing — the ×,
   Esc, a click on the backdrop — asks it to close through `close`.

   The body is only mounted while open, so anything half-done inside it (a
   pending confirmation) is gone the next time it opens. */
import { onMounted, ref, useId, watch } from 'vue'

const props = defineProps<{ open: boolean; title: string }>()
const emit = defineEmits<{ close: [] }>()

const dialog = ref<HTMLDialogElement | null>(null)
const titleId = useId()

function sync(): void {
  const element = dialog.value
  if (!element) return
  if (props.open && !element.open) element.showModal()
  else if (!props.open && element.open) element.close()
}

onMounted(sync)
watch(() => props.open, sync)

// A click whose press and release both land on the dialog element itself —
// outside the panel — is a click on the backdrop. Checking the press as well
// keeps a text selection dragged out of the panel from closing it.
let pressedBackdrop = false

function press(event: PointerEvent): void {
  pressedBackdrop = event.target === dialog.value
}

function click(event: MouseEvent): void {
  if (pressedBackdrop && event.target === dialog.value) emit('close')
  pressedBackdrop = false
}

// Esc closes the dialog natively. Whatever closed it, a close the parent did
// not ask for is reported so `open` never disagrees with what is on screen.
function closed(): void {
  if (props.open) emit('close')
}
</script>

<template>
  <dialog
    ref="dialog"
    class="modal"
    :aria-labelledby="titleId"
    @pointerdown="press"
    @click="click"
    @close="closed"
  >
    <div v-if="open" class="panel-body">
      <header class="head">
        <h2 :id="titleId" class="caps">{{ title }}</h2>
        <button class="btn btn-quiet close" type="button" aria-label="Close" @click="emit('close')">
          ×
        </button>
      </header>
      <div class="content scroll">
        <slot />
      </div>
    </div>
  </dialog>
</template>

<style scoped>
/* Native `dialog` sets its own display; while open it lays its panel out as
   a column so the head stays put and only the contents scroll. */
.modal[open] {
  display: flex;
}
/* A float like the popovers, so it takes their surface step — but it is the
   only one with a backdrop, and a scrim already says it is in front. So no
   Stone edge here: a hairline is enough, and a 600px panel outlined in Stone
   would be the loudest thing on the page.

   Inside a float on dark, a hairline on the page's terms is 1.08:1 and gone,
   which would cost the dialog's outline buttons their edge. `--hairline` is
   redefined for everything in here, this border included. */
.modal {
  width: min(600px, calc(100vw - 32px));
  max-width: 600px;
  max-height: calc(100vh - 32px);
  padding: 0;
  --hairline: var(--float-hairline);
  border: 1px solid var(--hairline);
  border-radius: var(--radius-surface);
  background: var(--surface-float);
  color: var(--text-body);
}
.modal::backdrop {
  background: rgb(0 0 0 / 0.6);
}
.panel-body {
  display: flex;
  flex-direction: column;
  gap: 32px;
  padding: 20px 24px 24px;
  min-width: 0;
  min-height: 0;
  flex: 1;
}
.head {
  flex: none;
}
/* Everything the dialog was given, scrolling inside what the head leaves;
   `.scroll` carries the overflow and the thin scrollbar. */
.content {
  min-height: 0;
}
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.head h2 {
  margin: 0;
  font-size: 20px;
}
.close {
  padding: 0 0px;
  font-size: 28px;
  line-height: 1;
}
</style>
