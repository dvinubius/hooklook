<script setup lang="ts">
/* What a visitor sees when `/` set its cookie check and the browser did not
   send it back. Go served this document with no bin and no owner cookie: a
   bin handed to a browser that drops cookies could never be found again.

   The one action is a plain link to `/`, which starts the check afresh — the
   way back in after the visitor allows cookies. */
import PageShell from './PageShell.vue'
</script>

<template>
  <PageShell>
    <main class="page-state">
      <div class="words">
        <h1 class="heading">Hooklook needs cookies</h1>
        <p class="note">
          Your bin belongs to this browser through a cookie. Without it, hooklook could not
          find your bin again, so it did not open one.
        </p>
        <p class="note">Allow cookies for this site, then try again.</p>
        <a class="btn btn-primary" href="/">Try again</a>
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
  align-items: center;
  gap: 12px;
}
.words .heading {
  line-height: var(--leading-heading);
}
.words .btn {
  margin-top: 8px;
}
</style>
