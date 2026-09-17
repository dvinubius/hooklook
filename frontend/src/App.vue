<script setup lang="ts">
/* Step 1 scaffold: proves the development loop end to end — the page shell is
   served and authorized by Go, the module graph comes from Vite on the same
   browser origin, and an authorized metadata call carries the owner cookie.
   Step 3 replaces this with the real bin bootstrap. */
import { onMounted, ref } from 'vue'
import { api, ApiError } from './api'
import { parseLocation } from './lib/location'
import { theme, toggleTheme, useThemeAttribute } from './lib/theme'
import type { BinAccess } from './types'

useThemeAttribute()

const page = parseLocation(window.location.href, window.location.origin)
const access = ref<BinAccess | null>(null)
const error = ref('')

onMounted(async () => {
  try {
    access.value = await api.binAccess(page.code, page.invite)
  } catch (cause) {
    error.value = cause instanceof ApiError ? `${cause.status} · ${cause.message}` : String(cause)
  }
})
</script>

<template>
  <main class="boot">
    <header class="boot-head">
      <div class="mark">
        <span class="bracket">[</span> hooklook <span class="bracket">]</span>
      </div>
      <button class="btn btn-quiet" type="button" @click="toggleTheme">
        // {{ theme === 'dark' ? 'dark' : 'light' }}
      </button>
    </header>

    <p class="meta">// development loop check</p>

    <dl v-if="access" class="readout">
      <dt>bin</dt>
      <dd>{{ access.bin.code }}</dd>
      <dt>role</dt>
      <dd>{{ access.owner ? 'owner' : 'guest' }}</dd>
      <dt>sharing</dt>
      <dd>{{ access.sharingEnabled ? 'enabled' : 'disabled' }}</dd>
      <dt>expires</dt>
      <dd>{{ access.bin.expiresAt }}</dd>
    </dl>
    <p v-else-if="error" class="mono failure">metadata call failed — {{ error }}</p>
    <p v-else class="mono muted">loading metadata…</p>
  </main>
</template>

<style scoped>
.boot {
  max-width: 720px;
  padding: 48px 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.boot-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
}
.mark {
  font-size: var(--text-heading);
  font-weight: 500;
  letter-spacing: var(--track-wordmark);
}
.bracket {
  color: var(--accent);
  font-weight: 700;
}
.readout {
  display: grid;
  grid-template-columns: 90px 1fr;
  gap: 8px 20px;
  margin: 0;
  font-family: var(--font-mono);
  font-size: var(--text-small);
}
.readout dt {
  color: var(--text-muted);
}
.readout dd {
  margin: 0;
}
.failure {
  font-size: var(--text-small);
  color: var(--text-body);
}
</style>
