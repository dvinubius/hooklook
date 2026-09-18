<script setup lang="ts">
/* The owner's settings, shown inside the settings modal, which supplies the
   title. This component is not rendered for a guest at all,
   and hiding it is presentation only: the server re-checks ownership, the
   cookie and the request origin on every one of these calls.

   Clearing, the one destructive action, asks first, in place, and says what it
   destroys — and what it keeps: the bin, its capture URL and its invitation. */
import { computed, ref, useId } from 'vue'
import LinkField from './LinkField.vue'
import { inviteUrl } from '../lib/location'
import type { BinAccess } from '../types'

const props = defineProps<{
  access: BinAccess
  origin: string
  requestCount: number
  busy: string
  error: string
}>()

const emit = defineEmits<{
  sharing: [enabled: boolean]
  clear: []
}>()

const accessLabel = useId()
const accessOptions = [
  { value: 'private', shared: false },
  { value: 'shared', shared: true },
] as const

const confirming = ref(false)

const link = computed(() =>
  props.access.inviteId ? inviteUrl(props.access.bin.code, props.access.inviteId, props.origin) : '',
)

function confirmClear(): void {
  if (!confirming.value) {
    confirming.value = true
    return
  }
  confirming.value = false
  emit('clear')
}
</script>

<template>
  <section class="owner">
    <div class="block">
      <div class="row">
        <span :id="accessLabel" class="label">Access</span>
        <div class="toggle" role="radiogroup" :aria-labelledby="accessLabel">
          <button
            v-for="option in accessOptions"
            :key="option.value"
            class="option"
            type="button"
            role="radio"
            :aria-checked="option.shared === access.sharingEnabled"
            :disabled="busy !== ''"
            @click="option.shared !== access.sharingEnabled && emit('sharing', option.shared)"
          >
            {{ option.value }}
          </button>
        </div>
        <span v-if="busy === 'sharing'" class="value">saving…</span>
      </div>
    </div>

    <div v-if="link" class="block">
      <span class="label">Guest link</span>
      <!-- The link only works while access is shared, so only then is it accented. -->
      <LinkField :url="link" copy-label="Copy guest link" :muted="!access.sharingEnabled" />
      <p class="note">
        In shared access mode, guests can view the bin page, but cannot change or delete anything.
      </p>
    </div>

    <div class="block">
      <div class="row">
        <span class="label">Captured requests</span>
        <span class="value">{{ requestCount }}</span>
        <button
          class="btn btn-danger btn-sm"
          type="button"
          :disabled="busy !== '' || requestCount === 0"
          @click="confirmClear"
        >
          {{
            busy === 'clear'
              ? 'Clearing…'
              : confirming
                ? `Confirm: delete all ${requestCount}`
                : 'Clear all requests'
          }}
        </button>
        <button
          v-if="confirming"
          class="btn btn-quiet btn-sm"
          type="button"
          @click="confirming = false"
        >
          cancel
        </button>
      </div>
      <p v-if="confirming" class="note">
        This deletes every captured request and frees the space they used. The bin, its capture URL
        and its invitation link all stay as they are.
      </p>
    </div>

    <p v-if="error" class="failure">{{ error }}</p>
  </section>
</template>

<style scoped>
.owner {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px 14px;
}
.label {
  font-size: var(--text-small);
  min-width: 10rem;
}
.value {
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  color: var(--text-muted);
  flex: 1;
  word-break: break-all;
}
/* Two square segments; the chosen one is filled. */
.toggle {
  display: inline-flex;
  border: 1px solid var(--hairline);
}
.option {
  padding: 6px 14px;
  border: 0;
  background: transparent;
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  cursor: pointer;
}
.option + .option {
  border-left: 1px solid var(--hairline);
}
.option[aria-checked='true'] {
  background: var(--surface-shade);
  color: var(--text-body);
  cursor: default;
}
.option:not([aria-checked='true']):not(:disabled):hover {
  color: var(--text-body);
}
.option:disabled {
  cursor: not-allowed;
}
.note {
  margin: 0;
  max-width: 68ch;
  font-size: var(--text-small);
  line-height: var(--leading-small);
  color: var(--text-muted);
}
.failure {
  margin: 0;
  padding: 12px 14px;
  background: var(--surface-shade);
  font-size: var(--text-small);
}
</style>
