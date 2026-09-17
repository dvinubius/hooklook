<script setup lang="ts">
/* Copying is reported in words: the brand's ✓ and × are a valence pair for
   figures and marked lists, not decoration on a control. */
import { onBeforeUnmount, ref } from 'vue'
import { copyText } from '../lib/clipboard'

const props = withDefaults(
  defineProps<{ text: string; label: string; variant?: 'primary' | 'outline' }>(),
  { variant: 'primary' },
)

const status = ref<'idle' | 'copied' | 'failed'>('idle')
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
    class="btn"
    :class="variant === 'outline' ? 'btn-outline' : 'btn-primary'"
    type="button"
    @click="copy"
  >
    <span aria-live="polite">
      {{ status === 'copied' ? 'Copied' : status === 'failed' ? 'Copy unavailable' : label }}
    </span>
  </button>
</template>
