<script setup lang="ts">
/* The owner's settings. This component is not rendered for a guest at all,
   and hiding it is presentation only: the server re-checks ownership, the
   cookie and the request origin on every one of these calls.

   The two destructive actions ask first, in place, and say what they destroy.
   Clearing keeps the bin — same capture URL, same invitation. Replacing does
   not: it is a different bin, and every link anyone holds stops working. */
import { computed, ref } from 'vue'
import CopyButton from './CopyButton.vue'
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
  replace: []
}>()

const confirming = ref<'clear' | 'replace' | ''>('')

const link = computed(() =>
  props.access.inviteId ? inviteUrl(props.access.bin.code, props.access.inviteId, props.origin) : '',
)

function confirm(action: 'clear' | 'replace'): void {
  if (confirming.value !== action) {
    confirming.value = action
    return
  }
  confirming.value = ''
  if (action === 'clear') emit('clear')
  else emit('replace')
}
</script>

<template>
  <section class="owner">
    <h2 class="meta-caps">Bin settings</h2>

    <div class="block">
      <div class="row">
        <span class="label">Sharing</span>
        <span class="value">{{ access.sharingEnabled ? 'enabled' : 'disabled' }}</span>
        <button
          class="btn btn-outline btn-sm"
          type="button"
          :disabled="busy !== ''"
          @click="emit('sharing', !access.sharingEnabled)"
        >
          {{ busy === 'sharing' ? 'Saving…' : access.sharingEnabled ? 'Disable sharing' : 'Enable sharing' }}
        </button>
      </div>

      <template v-if="link">
        <div class="row">
          <p class="code-surface link">{{ link }}</p>
          <CopyButton :text="link" label="Copy invitation link" variant="outline" />
        </div>
        <p class="note">
          Anyone with this link can read what arrives in this bin, but cannot change or delete
          anything. The link stays the same while sharing is off — it simply stops working, and
          works again the moment you turn sharing back on.
        </p>
      </template>
    </div>

    <div class="block">
      <div class="row">
        <span class="label">Captured requests</span>
        <span class="value">{{ requestCount }}</span>
        <button
          class="btn btn-danger btn-sm"
          type="button"
          :disabled="busy !== '' || requestCount === 0"
          @click="confirm('clear')"
        >
          {{
            busy === 'clear'
              ? 'Clearing…'
              : confirming === 'clear'
                ? `Confirm: delete all ${requestCount}`
                : 'Clear all requests'
          }}
        </button>
        <button
          v-if="confirming === 'clear'"
          class="btn btn-quiet btn-sm"
          type="button"
          @click="confirming = ''"
        >
          cancel
        </button>
      </div>
      <p v-if="confirming === 'clear'" class="note">
        This deletes every captured request and frees the space they used. The bin, its capture URL
        and its invitation link all stay as they are.
      </p>
    </div>

    <div class="block">
      <div class="row">
        <span class="label">This bin</span>
        <span class="value mono">{{ access.bin.code }}</span>
        <button
          class="btn btn-danger btn-sm"
          type="button"
          :disabled="busy !== ''"
          @click="confirm('replace')"
        >
          {{
            busy === 'replace'
              ? 'Replacing…'
              : confirming === 'replace'
                ? 'Confirm: replace this bin'
                : 'Replace bin'
          }}
        </button>
        <button
          v-if="confirming === 'replace'"
          class="btn btn-quiet btn-sm"
          type="button"
          @click="confirming = ''"
        >
          cancel
        </button>
      </div>
      <p v-if="confirming === 'replace'" class="note">
        You get a new bin with a new code, and this one is gone along with everything in it. The
        capture URL you have given out stops working, and so does the invitation link — anyone
        still using them will need the new ones.
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
.owner h2 {
  margin: 0;
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
.link {
  flex: 1 1 320px;
  margin: 0;
  padding: 9px 12px;
  color: var(--paper);
  overflow-x: auto;
  white-space: nowrap;
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
