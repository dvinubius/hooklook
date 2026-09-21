<script setup lang="ts" generic="T extends string">
/* A small group of square segments, one of them active. The active one is
   brightened — full-strength text on a lifted fill — never accented: it is a
   view choice, not an alert. Each segment is a toggle button, so screen
   readers hear which one is pressed. */
const model = defineModel<T>({ required: true })
defineProps<{ options: { value: T; label: string }[]; label: string }>()
</script>

<template>
  <div class="segments" role="group" :aria-label="label">
    <button
      v-for="option in options"
      :key="option.value"
      class="segment"
      :class="{ active: option.value === model }"
      type="button"
      :aria-pressed="option.value === model"
      @click="model = option.value"
    >
      {{ option.label }}
    </button>
  </div>
</template>

<style scoped>
/* Clipped, so the active segment's fill keeps to the softened corners. */
.segments {
  display: inline-flex;
  border: 1px solid var(--hairline);
  border-radius: var(--radius-control);
  overflow: hidden;
}
.segment {
  padding: 4px 12px;
  border: 0;
  background: transparent;
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  cursor: pointer;
}
.segment + .segment {
  border-left: 1px solid var(--hairline);
}
.segment.active {
  background: var(--surface-card);
  color: var(--text-body);
  cursor: default;
}
.segment:not(.active):hover {
  color: var(--text-body);
}
</style>
