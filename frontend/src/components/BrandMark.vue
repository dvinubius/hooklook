<script setup lang="ts">
/* The product mark: the webhook glyph with a magnifying glass over its
   bottom-right hook, beside the wordmark. Geometry and the reasoning behind
   the numbers live in docs/frontend/brand-mark.md — change them there first.

   The mark takes the font size it is given. Its box is baseline-aligned and
   pushed down by half its height less 0.343em, which puts its centre on the
   middle of the wordmark's ink rather than on the x-height midline. */
import { useId } from 'vue'

/* Unique per instance: two marks on one page must not share a mask. */
const uid = useId()
</script>

<template>
  <span class="brand-mark">
    <svg viewBox="1 0.5 25.44 25.44" aria-hidden="true" focusable="false">
      <defs>
        <!-- The arms pass under the ring: knocked out here, with a 0.9 gap. -->
        <mask :id="`${uid}-cut`" maskUnits="userSpaceOnUse" x="-8" y="-8" width="46" height="46">
          <rect x="-8" y="-8" width="46" height="46" fill="#fff" />
          <circle cx="18" cy="17" r="8.4" fill="#000" />
          <path d="M22.879 21.879L25.142 24.142" stroke="#000" stroke-width="4.4"
            stroke-linecap="round" />
        </mask>
        <!-- What shows through the glass. -->
        <clipPath :id="`${uid}-glass`"><circle cx="18" cy="17" r="5.2" /></clipPath>
      </defs>
      <g fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <g class="glyph" :mask="`url(#${uid}-cut)`">
          <path d="M18 16.98h-5.99c-1.1 0-1.95.94-2.48 1.9A4 4 0 0 1 2 17c.01-.7.2-1.4.57-2" />
          <path d="M6 17l3.13-5.78c.53-.97.1-2.18-.5-3.1a4 4 0 1 1 6.89-4.06" />
          <path d="M12 6l3.13 5.73C15.66 12.7 16.9 13 18 13a4 4 0 0 1 0 8" />
        </g>
        <g class="glyph behind" :clip-path="`url(#${uid}-glass)`">
          <path d="M18 16.98h-5.99c-1.1 0-1.95.94-2.48 1.9A4 4 0 0 1 2 17c.01-.7.2-1.4.57-2" />
          <path d="M6 17l3.13-5.78c.53-.97.1-2.18-.5-3.1a4 4 0 1 1 6.89-4.06" />
          <path d="M12 6l3.13 5.73C15.66 12.7 16.9 13 18 13a4 4 0 0 1 0 8" />
        </g>
        <circle class="lens" cx="18" cy="17" r="6.5" />
        <!-- Starts inside the ring's stroke, so its round cap stays out of the glass. -->
        <path class="lens" d="M22.879 21.879L25.142 24.142" stroke-width="2.6" />
      </g>
    </svg>
    <span class="word">hooklook</span>
  </span>
</template>

<style scoped>
.brand-mark {
  display: inline-flex;
  align-items: baseline;
  gap: 0.38em;
  font-weight: 500;
  letter-spacing: var(--track-wordmark);
  white-space: nowrap;
}
.brand-mark svg {
  width: 1.15em;
  height: 1.15em;
  flex: none;
  transform: translateY(calc(1.15em / 2 - 0.343em));
}
.glyph {
  stroke: var(--text-body);
}
.behind {
  opacity: 0.6;
}
.lens {
  stroke: var(--accent);
  fill: var(--accent-glyph);
}
</style>
