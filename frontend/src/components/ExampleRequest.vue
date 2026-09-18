<script setup lang="ts">
/* A request to try the capture URL with, on a code surface, with a copy
   control for the whole command. It is produced as token data and rendered
   through text interpolation, like every other code surface here. */
import { computed } from 'vue'
import CopyButton from './CopyButton.vue'

const props = defineProps<{ url: string }>()

type Tone = 'emphasis' | 'body' | 'recede'

// The capture URL is set once, so the request line stays short. Whatever is
// appended to it becomes the captured path and query.
const example = computed<{ text: string; tone: Tone }[][]>(() => [
  [
    { text: 'BASE_URL', tone: 'emphasis' },
    { text: '=', tone: 'recede' },
    { text: `'${props.url}'`, tone: 'body' },
  ],
  [],
  [
    { text: 'curl', tone: 'emphasis' },
    { text: ' -X ', tone: 'recede' },
    { text: 'POST', tone: 'body' },
    { text: ' "$BASE_URL/orders/42?retry=1"', tone: 'body' },
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

// What the copy control puts on the clipboard: the same command, as text.
const exampleText = computed(() =>
  example.value.map((line) => line.map((part) => part.text).join('')).join('\n'),
)
</script>

<template>
  <div class="example">
    <pre
      class="code-surface scroll code"
      aria-label="Example request"
    ><code><template v-for="(line, i) in example" :key="i"><span
      v-for="(part, j) in line" :key="j" :class="`tok-${part.tone}`">{{ part.text }}</span>{{ i < example.length - 1 ? '\n' : '' }}</template></code></pre>
    <CopyButton class="copy" :text="exampleText" label="Copy example request" variant="icon" muted />
  </div>
</template>

<style scoped>
.example {
  position: relative;
  min-width: 0;
}
.code {
  margin: 0;
  padding: 14px 16px;
  font-size: 12px;
  white-space: pre;
}
/* The copy control sits in the top-right corner, beside the short assignment
   line, so the lines below keep the full width. Its icon is inset exactly as
   far as the code is — 14px from the top, 16px from the side. */
.example .copy {
  position: absolute;
  top: 14px;
  right: 16px;
  height: auto;
  min-width: 0;
  padding: 0;
}
</style>
