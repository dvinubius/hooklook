<script setup lang="ts">
/* The About card, built the way zibs builds its own: a native `<dialog>` in
   the top layer that fades in and out over the backdrop. The parent owns
   `open`; the ×, Esc and a backdrop click all ask it to close through `close`,
   and the dialog only leaves the top layer once the fade has run.

   The fade is the one piece of motion the style guide allows; its duration is
   read from the stylesheet. */
import { onBeforeUnmount, onMounted, ref, useId, watch } from 'vue'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const dialog = ref<HTMLDialogElement | null>(null)
const titleId = useId()
const fading = ref(true)
let closeTimer: ReturnType<typeof setTimeout> | undefined

function fadeDurationMs(element: HTMLElement): number {
  return (parseFloat(getComputedStyle(element).transitionDuration) || 0) * 1000
}

function sync(): void {
  const element = dialog.value
  if (!element) return
  clearTimeout(closeTimer)
  if (props.open) {
    if (!element.open) {
      fading.value = true
      element.showModal()
    }
    // Two frames, so the transparent state is painted before it is lifted.
    requestAnimationFrame(() =>
      requestAnimationFrame(() => {
        if (props.open) fading.value = false
      }),
    )
  } else if (element.open) {
    fading.value = true
    closeTimer = setTimeout(() => element.close(), fadeDurationMs(element))
  }
}

onMounted(sync)
watch(() => props.open, sync)
onBeforeUnmount(() => clearTimeout(closeTimer))

// Esc: fade out instead of the instant native close.
function cancel(event: Event): void {
  event.preventDefault()
  emit('close')
}

// A click whose press and release both land on the dialog element itself —
// outside the card — is a click on the backdrop. Checking the press as well
// keeps a text selection dragged out of the card from closing it.
let pressedBackdrop = false

function press(event: PointerEvent): void {
  pressedBackdrop = event.target === dialog.value
}

function click(event: MouseEvent): void {
  if (pressedBackdrop && event.target === dialog.value) emit('close')
  pressedBackdrop = false
}

// The browser can still close the dialog on its own (a second Esc it refuses
// to let us cancel); report it so `open` never disagrees with the screen.
function closed(): void {
  fading.value = true
  if (props.open) emit('close')
}
</script>

<template>
  <dialog
    ref="dialog"
    class="about-dialog"
    :class="{ 'is-fading': fading }"
    :aria-labelledby="titleId"
    @cancel="cancel"
    @pointerdown="press"
    @click="click"
    @close="closed"
  >
    <div class="about-card">
      <div class="about-top">
        <h2 :id="titleId" class="about-title">about hooklook</h2>
        <button class="about-close" type="button" aria-label="Close" @click="emit('close')">
          ×
        </button>
      </div>
      <p class="about-lede">
        A request bin for testing webhook integrations.
        <br />Point a webhook at your capture URL and watch its requests arrive.
      </p>
      <p class="faq-q">// what happens to credentials in headers?</p>
      <p class="faq-a">
        Credential headers are replaced with [REDACTED] before storage. The original values were
        never written down and cannot be recovered here.
      </p>
    </div>
  </dialog>
</template>

<style scoped>
/* Brand card in the top layer: square, flat, no chrome, centered. Sizes
   match zibs, except the ×, which is set at 28px like the settings dialog's. */
.about-dialog {
  background: var(--surface-card);
  color: var(--text-body);
  border: none;
  border-radius: 0;
  padding: 0;
  width: min(520px, calc(100vw - 48px));
  overflow-y: auto;
  transition: opacity 340ms ease;
}
.about-dialog::backdrop {
  background: rgb(20 20 20 / 0.5); /* Ink-based scrim, both themes */
  transition: opacity 340ms ease;
}
.is-fading {
  opacity: 0;
}
.about-dialog.is-fading::backdrop {
  opacity: 0;
}
.about-card {
  padding: 32px;
}
.about-top {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
}
.about-title {
  margin: 0;
  font-size: 20px;
  font-weight: 500;
  letter-spacing: var(--track-heading);
}
.about-close {
  background: none;
  border: none;
  padding: 4px;
  font-size: 28px;
  line-height: 1;
  color: var(--text-muted);
  cursor: pointer;
}
.about-close:hover {
  color: var(--text-body);
}
.about-lede {
  margin: 32px 0 0;
  font-size: 15px;
}
/* Question-and-answer pairs, for the content still to come. */
.faq-q {
  margin: 24px 0 0;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-muted);
}
.faq-a {
  margin: 6px 0 0;
  font-size: 14px;
  line-height: 1.55;
}
</style>
