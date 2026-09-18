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
        <h2 :id="titleId" class="meta-caps">{{ title }}</h2>
        <button class="btn btn-quiet close" type="button" aria-label="Close" @click="emit('close')">
          ×
        </button>
      </header>
      <slot />
    </div>
  </dialog>
</template>

<style scoped>
.modal {
  width: min(500px, calc(100vw - 32px));
  max-width: 500px;
  max-height: calc(100vh - 32px);
  padding: 0;
  border: 1px solid var(--hairline);
  background: var(--surface-page);
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
