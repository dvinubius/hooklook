<script setup lang="ts">
/* The theme switch: a moon on light pages, a sun on dark ones — the icon
   names the theme a click switches to. Same glyphs as zibs. */
import { computed } from 'vue'
import { theme, toggleTheme } from '../lib/theme'

const label = computed(() =>
  theme.value === 'dark' ? 'Switch to light theme' : 'Switch to dark theme',
)
</script>

<template>
  <button class="theme-toggle" type="button" :aria-label="label" :title="label" @click="toggleTheme">
    <svg v-if="theme === 'dark'" viewBox="0 0 24 24" fill="none" stroke="currentColor"
      stroke-width="1.8" aria-hidden="true" focusable="false">
      <circle cx="12" cy="12" r="4" fill="currentColor" stroke="none" />
      <path
        d="M12 2v3M12 19v3M2 12h3M19 12h3M4.2 4.2l2.1 2.1M17.7 17.7l2.1 2.1M19.8 4.2l-2.1 2.1M6.3 17.7l-2.1 2.1"
      />
    </svg>
    <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"
      aria-hidden="true" focusable="false">
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
    </svg>
  </button>
</template>

<style scoped>
.theme-toggle {
  display: inline-flex;
  align-self: center;
  background: none;
  border: none;
  padding: 4px;
  color: var(--text-muted);
  cursor: pointer;
}
.theme-toggle:hover {
  color: var(--text-body);
}
.theme-toggle svg {
  width: 22.4px;
  height: 22.4px;
}
</style>
