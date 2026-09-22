<script setup lang="ts">
/* The frame every full page shares: the wordmark and the theme toggle at the
   top, the credits at the bottom, and whatever the page is about between
   them. Full-page states differ only in that middle, so the frame is here
   rather than copied into each of them.

   The slotted content is the flex child that takes the remaining height, so
   a page that scrolls scrolls inside itself and the footer stays on screen. */
import BrandMark from './BrandMark.vue'
import IconGithub from './IconGithub.vue'
import ThemeToggle from './ThemeToggle.vue'
</script>

<template>
  <div class="page">
    <header class="top">
      <a class="home" href="/" aria-label="hooklook home">
        <BrandMark class="mark" />
      </a>
      <div class="top-end">
        <ThemeToggle />
      </div>
    </header>
    <hr class="rule" />

    <slot />

    <!-- As in zibs: the personal wordmark and a quiet link home, with the
         credit — pointing at the source — between them. -->
    <footer class="foot">
      <p class="wordmark">
        <span class="bracket">[ </span>Dinu Barbu<span class="bracket"> ]</span>
      </p>
      <a class="credit" href="https://github.com/dvinubius/hooklook" title="hooklook on GitHub">
        ↳ dvinubius
        <IconGithub class="github" />
        <span class="sr-only">— hooklook on GitHub</span>
      </a>
      <a class="foot-link" href="https://dinubarbu.com">→ dinubarbu.com</a>
    </footer>
  </div>
</template>

<style scoped>
/* Exactly one viewport high, so the footer is always on screen: what the page
   puts between the bars takes what is left and scrolls inside it. */
.page {
  display: flex;
  flex-direction: column;
  height: 100%;
  max-width: 1180px;
  width: 100%;
  margin: 0 auto;
  padding: 16px 24px 14px;
  gap: 14px;
}
.top {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
}
.mark {
  font-size: 24px;
}
.home {
  display: inline-flex;
  color: inherit;
  text-decoration: none;
}
.bracket {
  color: var(--accent);
  font-weight: 700;
}
.top-end {
  display: flex;
  align-items: baseline;
  gap: 24px;
}
.foot {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: baseline;
  gap: 24px;
  padding-top: 14px;
  border-top: 1px solid var(--hairline);
}
.wordmark {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
  letter-spacing: var(--track-wordmark);
  white-space: nowrap;
}
/* Quiet link: body text over a 1px accent rule, with a leading arrow. */
/* The credit sits dead centre, whatever the two ends measure. */
.credit {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-family: var(--font-mono);
  font-size: var(--text-mono-micro);
  color: var(--text-muted);
  text-decoration: none;
}
.credit:hover {
  color: var(--text-body);
}
.github {
  width: 14px;
  height: 14px;
}
.foot-link {
  justify-self: end;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-body);
  text-decoration: none;
  border-bottom: 1px solid var(--accent);
  padding-bottom: 1px;
}
.foot-link:hover {
  color: var(--accent-on-hover);
}
</style>
