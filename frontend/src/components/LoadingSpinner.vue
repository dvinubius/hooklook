<script setup lang="ts">
/* A small turning arc for a pending action — one of the few moving things
   the style guide allows. It announces `label` to screen readers. */
defineProps<{ label: string }>()
</script>

<template>
  <span class="spinner" role="status">
    <svg viewBox="0 0 16 16" fill="none" aria-hidden="true" focusable="false">
      <circle class="track" cx="8" cy="8" r="6.5" />
      <path class="arc" d="M8 1.5A6.5 6.5 0 0 1 14.5 8" />
    </svg>
    <span class="sr-only">{{ label }}</span>
  </span>
</template>

<style scoped>
.spinner {
  display: inline-flex;
  width: 14px;
  height: 14px;
}
svg {
  width: 100%;
  height: 100%;
  animation: turn 0.8s linear infinite;
}
.track {
  stroke: var(--hairline);
  stroke-width: 1.5;
}
.arc {
  stroke: var(--text-muted);
  stroke-width: 1.5;
}
@keyframes turn {
  to {
    transform: rotate(360deg);
  }
}
</style>
