<script setup lang="ts">
/* A dead target or a shared URL whose access is invalid. The server marks the
   document before Vue starts, so this page asks no API and does not silently
   replace the bin. */
import PageShell from './PageShell.vue'

defineProps<{ kind: 'expired' | 'shared' }>()
</script>

<template>
  <PageShell>
    <main class="page-state">
      <div class="content">
        <p v-if="kind === 'expired'" class="comment message">
          // This bin has expired or no longer exists.
        </p>
        <p v-else class="comment message">// This shared bin no longer exists.</p>
        <a class="btn btn-primary" href="/">Create New Bin</a>
      </div>
    </main>
  </PageShell>
</template>

<style scoped>
.content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
  text-align: center;
}
.message {
  margin: 0;
}
</style>
