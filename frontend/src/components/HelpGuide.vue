<script setup lang="ts">
/* The owner's help dialog content, in five sections: sending requests to the
   capture URL, how much the bin holds, what happens to credentials in their
   headers, sharing the bin through its guest link, and keeping the bin. A
   guest gets no dialog: every section either tells them to do something that
   is not theirs to do, or says what the page already shows them.

   Sharing is only explained here; the switch and the link themselves live
   under the page's share button, as the capacity gauge lives on the capture
   row — this says what the numbers there mean. */
import ExampleRequest from './ExampleRequest.vue'
import IconShare from './IconShare.vue'

defineProps<{ captureUrl: string }>()
</script>

<template>
  <div class="guide">
    <section class="section">
      <h3 class="meta-caps">capture requests</h3>
      <p class="note">
        Send your webhooks to the bin using its public link as a base URL:
      </p>
      <ExampleRequest :url="captureUrl" />
    </section>

    <section class="section">
      <h3 class="meta-caps">bin capacity</h3>
      <p class="note">
        Your bin has a limited storage capacity — roughly <strong class="key">100MB</strong>, and at
        most <strong class="key">500 requests</strong>. Once it fills up you can delete individual
        requests or clear the entire bin.
      </p>
    </section>

    <section class="section">
      <h3 class="meta-caps">credentials in headers</h3>
      <p class="note">
        Credential headers are replaced with <strong class="key redacted">[REDACTED]</strong> and never stored.
      </p>
    </section>

    <section class="section">
      <h3 class="meta-caps">share the bin</h3>
      <p class="note">
        You can share this bin with colleagues. As guests, they can access and inspect captured
        requests, but not to delete any.
      </p>
      <p class="note">
        The share button <IconShare class="inline-icon" /> holds the guest link. It is only
        valid when you <strong class="key">enable guest access</strong> there.
      </p>
    </section>

    <section class="section">
      <h3 class="meta-caps">Preserve the Bin</h3>
      <p class="note">
        Keep using your bin in order to preserve it. As long as it captures new requests or you are
        inspecting it, the bin stays persistent. Inspection by guests also prolongs its life.
      </p>
      <p class="note">
        It survives <strong class="key">3 days</strong> of inactivity, then it's deleted.
      </p>
      <p class="note">After a bin deletion, when you come back you're getting a new bin.</p>
    </section>
  </div>
</template>

<style scoped>
.guide {
  display: flex;
  flex-direction: column;
  gap: 32px;
  padding-right: 2px;
}
.section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.section h3 {
  margin: 0;
  font-size: 14px;
}
/* The figures and the one condition a reader must not miss stand out from
   the notes around them by weight, as the brand does — not by the accent,
   which never goes on running text, and no longer by a lift out of the muted
   grey: the notes are not muted any more, so there is nothing to lift out of.

   Weight alone carries it as dark ink on a light page, where a heavier stroke
   is plainly more ink. Light text on a dark page blooms, so the same step
   barely tells, and dark mode adds brightness as a second channel: body text
   against the dim tier the notes are set in, 1.96x their contrast. That is a
   lift out of the prose colour, which the page otherwise avoids — but what
   made it wrong before was that every note was muted, so the lift was the
   only thing marking prose worth reading. The notes sit at the reading tier
   now, and this is a step above it rather than a substitute for it. Light
   needs no second channel and does not take one.

   The weight is the brand's medium, the same step headings take. */
.key {
  font-weight: 500;
}
[data-theme="dark"] .key {
  color: var(--text-body);
}
/* Names a control by drawing it: the icon rides the line at text size. */
.inline-icon {
  width: 1.15em;
  height: 1.15em;
  vertical-align: -0.22em;
  color: var(--text-body);
}
/* The marker as the headers list shows it. */
.key.redacted {
  color: var(--violet);
}
/* The guide is the app's only explanatory prose, and it is here to be read:
   it sits at the reading tier, not the incidental one. */
.note {
  margin: 0;
  font-size: var(--text-small);
  line-height: var(--leading-small);
  color: var(--text-dim);
}
</style>
