<script setup lang="ts">
/* What a first-time visitor sees when the store has no room for another bin.
   Go served this document with `507` and no cookie, so there is no bin, no
   code and nothing to authorize — only an apology, centred between the bars.

   No retry button: the visitor cannot free the space, and a button that
   reloads into the same wall is worse than the plain sentence. Reloading the
   page is still the way back in once room is freed, and that needs no control
   of ours. */
import PageShell from './PageShell.vue'
</script>

<template>
  <PageShell>
    <main class="page-state">
      <div class="words">
        <h1 class="heading">The service is at capacity</h1>
        <p class="note">
          hooklook has run out of room, so it cannot open a bin for you right now.
          Sorry about that — please try again later.
        </p>
      </div>
    </main>
  </PageShell>
</template>

<style scoped>
.words {
  max-width: 34rem;
  text-align: center;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.words .heading {
  line-height: var(--leading-heading);
}
</style>
