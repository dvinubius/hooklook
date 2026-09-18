<script setup lang="ts">
/* Copying is reported in words: the brand's ✓ and × are a valence pair for
   figures and marked lists, not decoration on a control. The icon variant
   sits inside a field, so it shows the copy icon until there is something to
   say, and then says it in the icon's place. */
import { computed, onBeforeUnmount, ref } from 'vue'
import IconCopy from './IconCopy.vue'
import { copyText } from '../lib/clipboard'

const props = withDefaults(
  defineProps<{
    text: string
    label: string
    variant?: 'primary' | 'outline' | 'icon'
    /** Icon variant only: muted instead of accent, for a link not in use. */
    muted?: boolean
  }>(),
  { variant: 'primary', muted: false },
)

const status = ref<'idle' | 'copied' | 'failed'>('idle')
const said = computed(() =>
  status.value === 'copied' ? 'Copied' : status.value === 'failed' ? 'Copy unavailable' : '',
)
let reset: ReturnType<typeof setTimeout> | undefined

async function copy(): Promise<void> {
  status.value = (await copyText(props.text)) ? 'copied' : 'failed'
  clearTimeout(reset)
  reset = setTimeout(() => (status.value = 'idle'), 3000)
}

onBeforeUnmount(() => clearTimeout(reset))
</script>

<template>
  <button
    v-if="variant === 'icon'"
    class="copy-icon"
    :class="{ muted }"
    type="button"
    :aria-label="label"
    :title="status === 'idle' ? label : undefined"
    @click="copy"
  >
    <IconCopy v-if="status === 'idle'" class="glyph" />
    <span aria-live="polite" class="said">{{ said }}</span>
  </button>
  <button
    v-else
    class="btn"
    :class="variant === 'outline' ? 'btn-outline' : 'btn-primary'"
    type="button"
    @click="copy"
  >
    <span aria-live="polite">
      {{ said || label }}
    </span>
  </button>
</template>

<style scoped>
.copy-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
  min-width: 42px;
  height: 100%;
  padding: 0 8px;
  border: 0;
  background: transparent;
  color: var(--accent);
  cursor: pointer;
}
.copy-icon:hover {
  color: var(--accent-on-hover);
}
.copy-icon.muted {
  color: var(--text-muted);
}
.copy-icon.muted:hover {
  color: var(--text-body);
}
[data-theme="dark"] .copy-icon:hover {
  color: var(--paper);
}
.glyph {
  width: 26px;
  height: 26px;
}
.said {
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  white-space: nowrap;
}
</style>
