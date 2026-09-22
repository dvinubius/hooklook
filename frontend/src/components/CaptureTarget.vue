<script setup lang="ts">
/* The public capture URL. It is an address anyone may send to — deliberately
   not a way in: inspection needs the owner cookie or an invitation, and this
   copy must not suggest otherwise.

   One row: whatever heading the page puts in the `heading` slot, the link
   with its copy control inside the field and the page's `tools` right beside
   it, and the page's `actions` at the far right. A request to try the URL
   with lives in the help dialog (`ExampleRequest`). */
import { computed } from 'vue'
import LinkField from './LinkField.vue'
import { captureUrl } from '../lib/location'

const props = defineProps<{ code: string; origin: string }>()

const url = computed(() => captureUrl(props.code, props.origin))
</script>

<template>
  <section class="capture">
    <slot name="heading" />
    <div class="link-group">
      <LinkField class="link" :url="url" :base="origin" copy-label="Copy capture URL" />
      <slot name="tools" />
    </div>
    <div v-if="$slots.actions" class="actions">
      <slot name="actions" />
    </div>
  </section>
</template>

<style scoped>
.capture {
  display: flex;
  align-items: center;
  gap: 22px;
}
/* The link and the tools that act on the bin sit together, 8px apart. */
.link-group {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.link {
  flex: 0 1 auto;
  width: 400px;
  min-width: 0;
}
.actions {
  margin-left: auto;
}
/* The copy icon matches the row's other icons, a step smaller than the
   copy control's default. */
.link :deep(.glyph) {
  width: 20px;
  height: 20px;
}
</style>
