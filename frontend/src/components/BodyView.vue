<script setup lang="ts">
/* The captured body.
   Everything rendered here goes through text interpolation — the highlighter
   hands over tokens with a brightness tier, never markup — so a body that
   contains `<script>` or an unclosed tag is shown as the characters it is.
   Nothing is reformatted silently either: when the raw and formatted views
   differ, the reader chooses, and a body that failed to parse says so while
   keeping the raw view intact. Copying always takes the raw text, whichever
   view is showing. */
import { computed, ref, watch } from 'vue'
import CopyButton from './CopyButton.vue'
import InfoPopover from './InfoPopover.vue'
import SegmentedControl from './SegmentedControl.vue'
import { formatBytes, hexDump, highlight, type DecodedBody } from '../lib/body'

const props = defineProps<{ body: DecodedBody }>()

type View = 'raw' | 'formatted'
const views: { value: View; label: string }[] = [
  { value: 'raw', label: 'raw' },
  { value: 'formatted', label: 'formatted' },
]
const view = ref<View>('formatted')

// Formatted by default; a new request may not offer the view the previous
// one was showing.
watch(
  () => props.body,
  (body) => {
    view.value = body.formatted !== null ? 'formatted' : 'raw'
  },
  { immediate: true },
)

const showingFormatted = computed(() => view.value === 'formatted' && props.body.formatted !== null)
const shown = computed(() => (showingFormatted.value ? props.body.formatted! : props.body.text))

const tokens = computed(() =>
  highlight(shown.value, showingFormatted.value ? props.body.format : 'text'),
)

const dump = computed(() => hexDump(props.body.bytes))
</script>

<template>
  <section class="body-view">
    <header class="head">
      <div class="title">
        <h2 class="meta-caps">Body</h2>
        <InfoPopover label="About request bodies">
          Bodies are stored exactly as received, without redaction. They may contain secrets or
          personal data; use test data and share this bin carefully.
        </InfoPopover>
      </div>
      <span class="separator" aria-hidden="true"></span>
      <span class="micro">
        {{ formatBytes(body.size) }}
        <template v-if="body.kind === 'text'">· {{ body.format }} · {{ body.encoding }}</template>
        <template v-else-if="body.kind === 'binary'">· binary</template>
      </span>
      <div v-if="body.kind === 'text' && body.formatted !== null" class="head-actions">
        <SegmentedControl v-model="view" :options="views" label="Body view" />
      </div>
    </header>

    <p v-if="body.kind === 'empty'" class="code-surface meta empty">// no body</p>

    <template v-else-if="body.kind === 'binary'">
      <p class="note">
        These bytes are not text, so they are shown as bytes. Offsets are hexadecimal; the column on
        the right is the printable ASCII of each row.
      </p>
      <pre class="code-surface scroll dump">{{ dump.lines.join('\n') }}</pre>
      <p v-if="dump.truncated" class="micro">
        ↳ Showing the first {{ formatBytes(dump.shown) }} of {{ formatBytes(body.size) }}.
      </p>
    </template>

    <template v-else>
      <p v-if="body.fellBack" class="note">
        These bytes are not valid {{ body.encoding === 'windows-1252' ? 'UTF-8' : 'text in the declared encoding' }},
        so they were decoded as {{ body.encoding }}. The bytes themselves are unchanged.
      </p>
      <p v-if="body.formatNote" class="note">{{ body.formatNote }}</p>
      <div class="text-block">
        <pre class="code-surface scroll text"><span
          v-for="(token, index) in tokens"
          :key="index"
          :class="`tok-${token.tier}`"
        >{{ token.text }}</span></pre>
        <CopyButton class="copy" :text="body.text" label="Copy raw body" variant="icon" muted />
      </div>
    </template>
  </section>
</template>

<style scoped>
.body-view {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}
.head {
  display: flex;
  align-items: baseline;
  gap: 12px;
  flex-wrap: wrap;
}
.title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
/* The facts beside the label read at 12px, a step above micro. */
.head .micro {
  font-size: var(--text-mono-meta);
}
.head h2 {
  margin: 0;
  font-size: 14px;
}
.head-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 16px;
}
.note {
  margin: 0;
  font-size: var(--text-small);
  line-height: var(--leading-small);
  color: var(--text-muted);
  max-width: 68ch;
}
/* No body is still shown where a body would be: the aside, centred in an
   empty code surface, at the surface's recede tier. */
.empty {
  margin: 0;
  padding: 28px 16px;
  text-align: center;
  /* The aside's own size; `.code-surface` would otherwise set its own. */
  font-size: var(--text-mono-meta);
  color: var(--code-recede);
}
pre {
  margin: 0;
  padding: 14px 16px;
  max-height: 28rem;
  white-space: pre;
  tab-size: 2;
}
.text {
  white-space: pre-wrap;
  word-break: break-word;
  /* Room on the right for the copy control, so no line runs under it. */
  padding-right: 50px;
}
/* The copy control stays in the block's top-right corner while the text
   scrolls, inset as far as the text is — as in the example request. */
.text-block {
  position: relative;
}
.text-block .copy {
  position: absolute;
  top: 14px;
  right: 16px;
  height: auto;
  min-width: 0;
  padding: 0;
}
</style>
