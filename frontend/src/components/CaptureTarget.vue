<script setup lang="ts">
/* The public capture URL. It is an address anyone may send to — deliberately
   not a way in: inspection needs the owner cookie or an invitation, and this
   copy must not suggest otherwise. */
import { computed } from 'vue'
import CopyButton from './CopyButton.vue'
import { captureUrl } from '../lib/location'

const props = defineProps<{ code: string; origin: string }>()

const url = computed(() => captureUrl(props.code, props.origin))
</script>

<template>
  <section class="capture">
    <h2 class="meta-caps">Capture URL</h2>

    <div class="row">
      <p class="code-surface target">{{ url }}</p>
      <CopyButton :text="url" label="Copy URL" />
    </div>

    <p class="note">
      Send it any method. Whatever you append becomes the captured path and query —
      <span class="example">{{ url }}/orders/42?retry=1</span>
    </p>
    <p class="note muted">
      Anyone holding this URL can send requests to your bin. It does not let them read
      what arrives here.
    </p>
  </section>
</template>

<style scoped>
.capture {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.row {
  display: flex;
  align-items: stretch;
  gap: 12px;
  flex-wrap: wrap;
}
.target {
  flex: 1 1 320px;
  margin: 0;
  padding: 10px 14px;
  color: var(--paper);
  overflow-x: auto;
  white-space: nowrap;
  display: flex;
  align-items: center;
}
.note {
  margin: 0;
  font-size: var(--text-small);
  line-height: var(--leading-small);
  max-width: 62ch;
}
.example {
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  color: var(--text-muted);
  white-space: nowrap;
}
</style>
