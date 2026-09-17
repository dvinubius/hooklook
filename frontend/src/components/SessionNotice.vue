<script setup lang="ts">
/* The page before — or instead of — an authorized bin. It never shows private
   data, so it says only what the browser is doing and, when that stalled, why. */
import type { SessionState } from '../lib/session'

defineProps<{ state: SessionState; message: string }>()
defineEmits<{ retry: [] }>()
</script>

<template>
  <main class="notice">
    <div class="mark"><span class="bracket">[</span> hooklook <span class="bracket">]</span></div>

    <p v-if="state === 'loading'" class="meta">// opening bin…</p>
    <p v-else-if="state === 'leaving'" class="meta">// taking you to your own bin…</p>

    <template v-else>
      <h1 class="heading">Bin unavailable</h1>
      <p class="reason">{{ message }}</p>
      <div class="actions">
        <button class="btn btn-outline" type="button" @click="$emit('retry')">Try again</button>
        <a class="btn btn-quiet" href="/">Go to your own bin →</a>
      </div>
    </template>
  </main>
</template>

<style scoped>
.notice {
  max-width: 34rem;
  margin: 0 auto;
  padding: 18vh 24px 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.mark {
  font-size: var(--text-title);
  font-weight: 500;
  letter-spacing: var(--track-wordmark);
  margin-bottom: 10px;
}
.bracket {
  color: var(--accent);
  font-weight: 700;
}
.reason {
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-small);
  line-height: var(--leading-small);
}
.actions {
  display: flex;
  align-items: center;
  gap: 18px;
  margin-top: 6px;
}
</style>
