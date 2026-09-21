<script setup lang="ts">
/* The owner's share control: a square icon button that opens a panel under
   it, right edges aligned, with the guest access switch and the guest link.
   Built on the native Popover API like `InfoPopover`: the browser toggles it
   from the button and closes it on Esc or a click elsewhere.

   The switch shows what the server holds: it flips once the change is saved,
   not on the click. */
import { onBeforeUnmount, ref, useId } from 'vue'
import IconShare from './IconShare.vue'
import LinkField from './LinkField.vue'
import LoadingSpinner from './LoadingSpinner.vue'
import ToggleSwitch from './ToggleSwitch.vue'

defineProps<{ enabled: boolean; saving: boolean; guestLink: string; origin: string }>()
const emit = defineEmits<{ toggle: [enabled: boolean] }>()

const id = useId()
const trigger = ref<HTMLButtonElement | null>(null)
const panel = ref<HTMLElement | null>(null)

const width = 420
const gutter = 16

// The panel is placed once, so a resize would leave it behind; it closes instead.
function dismiss(): void {
  panel.value?.hidePopover()
}

function place(event: Event): void {
  if ((event as ToggleEvent).newState !== 'open') {
    window.removeEventListener('resize', dismiss)
    return
  }
  window.addEventListener('resize', dismiss)
  const button = trigger.value?.getBoundingClientRect()
  const element = panel.value
  if (!button || !element) return
  const left = Math.max(gutter, Math.min(button.right - width, window.innerWidth - width - gutter))
  element.style.top = `${button.bottom + 8}px`
  element.style.left = `${left}px`
}

onBeforeUnmount(() => window.removeEventListener('resize', dismiss))
</script>

<template>
  <button
    ref="trigger"
    class="btn btn-outline icon-button"
    type="button"
    aria-label="Share"
    title="Share"
    :popovertarget="id"
  >
    <IconShare class="icon" />
  </button>
  <div :id="id" ref="panel" class="panel" popover @beforetoggle="place">
    <div class="access">
      <LoadingSpinner v-if="saving" label="Saving access" />
      <ToggleSwitch
        :model-value="enabled"
        label="Guest access"
        :disabled="saving"
        @update:model-value="emit('toggle', $event)"
      />
    </div>
    <div v-if="guestLink" class="link">
      <p class="label">Guest link</p>
      <LinkField class="guest-field" :url="guestLink" :base="origin" copy-label="Copy guest link" />
    </div>
  </div>
</template>

<style scoped>
/* Square, as high as the link field beside it — the same size as the page's
   other icon buttons. */
.icon-button {
  flex: none;
  width: var(--row-height, 44px);
  height: var(--row-height, 44px);
  padding: 0;
  justify-content: center;
}
.icon {
  width: 20px;
  height: 20px;
}
/* Fixed to the viewport under the button; flat, as the brand has no shadows,
   so a surface step and a Stone edge do the floating instead of one. */
.panel {
  position: fixed;
  inset: auto;
  width: 420px; /* = width in the script */
  max-width: calc(100vw - 32px);
  margin: 0;
  padding: 16px;
  border: 1px solid var(--text-muted);
  border-radius: var(--radius-surface);
  background: var(--surface-float);
  color: var(--text-body);
  flex-direction: column;
  gap: 16px;
}
.panel:popover-open {
  display: flex;
}
/* The switch sits at the panel's right edge; the spinner appears to the left
   of its label, so nothing moves while a change is saving. */
.access {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 16px;
}
.link {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
/* A smaller field than the page's capture link, to suit the panel, with the
   copy icon at the capture row's 20px. Qualified by `.link` so it wins over
   the field's own rules. */
.link .guest-field {
  --row-height: 36px;
  padding: 0 0 0 10px;
  font-size: 11px;
}
.link .guest-field :deep(.glyph) {
  width: 20px;
  height: 20px;
}
/* Set like the switch's own label: a control label, at the reading tier. */
.label {
  margin: 0;
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  color: var(--text-dim);
}
</style>
