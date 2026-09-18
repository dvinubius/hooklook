<script setup lang="ts">
/* The public capture URL. It is an address anyone may send to — deliberately
   not a way in: inspection needs the owner cookie or an invitation, and this
   copy must not suggest otherwise.

   Left, the link with its copy control inside the field, and whatever the page
   puts in the slot (the bin's facts) — rows of one height and width. Right, a
   request to try it with. */
import { computed } from 'vue'
import LinkField from './LinkField.vue'
import { captureUrl } from '../lib/location'

const props = defineProps<{ code: string; origin: string }>()

const url = computed(() => captureUrl(props.code, props.origin))

type Tone = 'emphasis' | 'body' | 'recede'

// Whatever is appended to the URL becomes the captured path and query.
const example = computed<{ text: string; tone: Tone }[][]>(() => [
  [
    { text: 'curl', tone: 'emphasis' },
    { text: ' -X ', tone: 'recede' },
    { text: 'POST', tone: 'body' },
    { text: ` '${url.value}/orders/42?retry=1'`, tone: 'body' },
    { text: ' \\', tone: 'recede' },
  ],
  [
    { text: '  -H ', tone: 'recede' },
    { text: `'Content-Type: application/json'`, tone: 'body' },
    { text: ' \\', tone: 'recede' },
  ],
  [
    { text: '  -d ', tone: 'recede' },
    { text: `'{"status":"paid"}'`, tone: 'body' },
  ],
])
</script>

<template>
  <section class="capture">
    <div class="rows">
      <LinkField :url="url" copy-label="Copy capture URL" />
      <slot />
    </div>

    <pre
      class="code-surface scroll example"
      aria-label="Example request"
    ><code><template v-for="(line, i) in example" :key="i"><span
      v-for="(part, j) in line" :key="j" :class="`tok-${part.tone}`">{{ part.text }}</span>{{ i < example.length - 1 ? '\n' : '' }}</template></code></pre>
  </section>
</template>

<style scoped>
.capture {
  display: grid;
  grid-template-columns: var(--lead-width, 400px) minmax(0, 1fr);
  gap: 32px;
  align-items: stretch;
}
@media (max-width: 900px) {
  .capture {
    grid-template-columns: minmax(0, var(--lead-width, 400px));
  }
}
.rows {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}
.example {
  margin: 0;
  padding: 14px 16px;
  white-space: pre;
}
</style>
