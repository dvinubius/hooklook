<script setup lang="ts">
/* Stored headers, exactly as stored. Some values were replaced before they
   ever reached the database; those are marked as redacted and left that way —
   there is nothing here that could reconstruct them, and nothing that tries. */
import { computed } from 'vue'

const props = defineProps<{ headers: Record<string, string[]> }>()

const redactedMarker = '[REDACTED]'

const rows = computed(() =>
  Object.entries(props.headers)
    .sort(([left], [right]) => (left.toLowerCase() < right.toLowerCase() ? -1 : 1))
    .map(([name, values]) => ({
      name,
      values: values.map((value) => ({ value, redacted: value === redactedMarker })),
    })),
)

const anyRedacted = computed(() =>
  rows.value.some((row) => row.values.some((entry) => entry.redacted)),
)
</script>

<template>
  <div class="headers">
    <p v-if="rows.length === 0" class="meta">// no headers were stored</p>

    <dl v-else class="table code-surface scroll">
      <template v-for="row in rows" :key="row.name">
        <dt class="name">{{ row.name }}</dt>
        <dd class="values">
          <span
            v-for="(entry, index) in row.values"
            :key="index"
            class="value"
            :class="entry.redacted ? 'tok-recede' : 'tok-body'"
          >{{ entry.value }}</span>
        </dd>
      </template>
    </dl>

    <p v-if="anyRedacted" class="micro note">
      ↳ Credential headers are replaced with {{ redactedMarker }} before storage. The original
      values were never written down and cannot be recovered here.
    </p>
  </div>
</template>

<style scoped>
.headers {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.table {
  display: grid;
  grid-template-columns: minmax(8rem, max-content) 1fr;
  gap: 2px 18px;
  margin: 0;
  padding: 14px 16px;
  max-height: 22rem;
  font-size: var(--text-mono-meta);
}
.name {
  color: var(--paper);
  white-space: nowrap;
}
.values {
  margin: 0;
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.value {
  word-break: break-all;
}
.note {
  margin: 0;
  max-width: 62ch;
  line-height: var(--leading-small);
}
</style>
