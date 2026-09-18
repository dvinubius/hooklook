<script setup lang="ts">
/* The owner's help dialog content, in three sections: sending requests to
   the capture URL, sharing the bin through its guest link, and keeping the
   bin. The guest link is shown whether or not guest access is on, and says
   that it only works while it is. */
import ExampleRequest from './ExampleRequest.vue'
import LinkField from './LinkField.vue'

defineProps<{ captureUrl: string; guestLink: string; origin: string }>()
</script>

<template>
  <div class="guide">
    <section class="section">
      <h3 class="meta-caps">capture requests</h3>
      <p class="note">
        Send your webhooks to the bin using its public link as a base URL. Try it out:
      </p>
      <ExampleRequest :url="captureUrl" />
      <p class="note">
        Your bin has a limited storage capacity — roughly <strong class="key">100MB</strong>, and at
        most <strong class="key">500 requests</strong>. Once it fills up you can delete individual
        requests or clear the entire bin.
      </p>
    </section>

    <section v-if="guestLink" class="section">
      <h3 class="meta-caps">share the bin</h3>
      <p class="note">
        You can share this bin with colleagues. As guests, they can access and inspect captured
        requests, but not to delete any.
      </p>
      <p class="note">
        This link is only valid when you <strong class="key">enable guest access</strong>.
      </p>
      <LinkField :url="guestLink" :base="origin" copy-label="Copy guest link" />
    </section>

    <section class="section">
      <h3 class="meta-caps">Preserve the Bin</h3>
      <p class="note">
        Keep using your bin in order to preserve it. As long as it captures new requests or you are
        inspecting it, the bin is persistent.
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
   the muted notes by brightness, as the brand does — not by the accent,
   which never goes on running text. */
.key {
  color: var(--text-body);
  font-weight: 500;
}
.note {
  margin: 0;
  font-size: var(--text-small);
  line-height: var(--leading-small);
  color: var(--text-muted);
}
</style>
