<script setup lang="ts">
/* A URL in a code field with its copy control inside, at the right end. The
   URL stays selectable text, so a refused clipboard still leaves a way to
   copy it.

   `base`, when the URL starts with it, recedes — it is the same for every
   link hooklook shows, so what follows it is what tells one link from
   another. The copy control and the tooltip still carry the whole URL. */
import { computed } from 'vue'
import CopyButton from './CopyButton.vue'

const props = withDefaults(
  defineProps<{ url: string; copyLabel: string; base?: string }>(),
  { base: '' },
)

const parts = computed(() =>
  props.base !== '' && props.url.startsWith(props.base)
    ? { base: props.base, rest: props.url.slice(props.base.length) }
    : { base: '', rest: props.url },
)
</script>

<template>
  <div class="code-surface link-field">
    <p class="url" :title="url"><span v-if="parts.base" class="tok-recede">{{ parts.base }}</span>{{ parts.rest }}</p>
    <CopyButton :text="url" :label="copyLabel" variant="icon" />
  </div>
</template>

<style scoped>
/* The URL is centred as a box of its own height rather than by a line-height
   equal to the field's, which set the text low. The copy control keeps the
   field's full height. */
/* Not `.field`: that is the global form-input style in app.css. The box
   holds the only padding; the text has none of its own. */
.link-field {
  display: flex;
  align-items: center;
  gap: 4px;
  height: var(--row-height, 44px);
  padding: 0 1px 0 14px;
}
.url {
  flex: 1 1 auto;
  min-width: 0;
  margin: 0;
  color: var(--paper);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
