<script setup lang="ts">
/* The captured body.
   Everything rendered here goes through text interpolation — the highlighter
   hands over tokens with a brightness tier, never markup — so a body that
   contains `<script>` or an unclosed tag is shown as the characters it is.
   Nothing is reformatted silently either: when the raw and formatted views
   differ, the reader chooses, and a body that failed to parse says so while
   keeping the raw view intact. */
import { computed, ref, watch } from 'vue'
import CopyButton from './CopyButton.vue'
import { formatBytes, hexDump, highlight, type DecodedBody } from '../lib/body'

const props = defineProps<{ body: DecodedBody }>()

const view = ref<'formatted' | 'raw'>('formatted')

// A new request may not offer the view the previous one was showing.
watch(
  () => props.body,
  (body) => {
    view.value = body.formatted === null ? 'raw' : 'formatted'
  },
  { immediate: true },
)

const shown = computed(() =>
  view.value === 'formatted' && props.body.formatted !== null ? props.body.formatted : props.body.text,
)

const tokens = computed(() =>
  highlight(shown.value, view.value === 'formatted' ? props.body.format : 'text'),
)

const dump = computed(() => hexDump(props.body.bytes))
</script>

<template>
  <section class="body-view">
    <header class="head">
      <h2 class="meta-caps">Body</h2>
      <span class="micro">
        {{ formatBytes(body.size) }}
        <template v-if="body.kind === 'text'">· {{ body.format }} · {{ body.encoding }}</template>
        <template v-else-if="body.kind === 'binary'">· binary</template>
      </span>
      <div class="head-actions">
        <template v-if="body.kind === 'text' && body.formatted !== null">
          <button
            class="btn btn-quiet btn-sm"
            type="button"
            @click="view = view === 'formatted' ? 'raw' : 'formatted'"
          >
            // show {{ view === 'formatted' ? 'raw' : 'formatted' }}
          </button>
        </template>
        <CopyButton v-if="body.kind === 'text'" :text="body.text" label="Copy body" variant="outline" />
      </div>
    </header>

    <p v-if="body.kind === 'empty'" class="meta">// this request had no body</p>

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
      <pre class="code-surface scroll text"><span
        v-for="(token, index) in tokens"
        :key="index"
        :class="`tok-${token.tier}`"
      >{{ token.text }}</span></pre>
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
.head h2 {
  margin: 0;
}
.head-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 14px;
}
.note {
  margin: 0;
  font-size: var(--text-small);
  line-height: var(--leading-small);
  color: var(--text-muted);
  max-width: 68ch;
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
}
</style>
