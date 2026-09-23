<script setup lang="ts">
/* Emptying the bin, the one destructive action on the whole bin, asks first
   and says what it destroys — and what it keeps: the bin, its capture URL and
   its guest link. A failure is reported here, in place, and the dialog stays
   open so it can be retried. */
defineProps<{ requestCount: number; clearing: boolean; error: string }>()
defineEmits<{ confirm: []; cancel: [] }>()
</script>

<template>
  <div class="confirm">
    <p class="note">
      This deletes all {{ requestCount }} captured requests and frees the space they used. The bin,
      its capture URL and its guest link all stay as they are.
    </p>
    <div class="actions">
      <button class="btn btn-outline btn-sm" type="button" @click="$emit('cancel')">Cancel</button>
      <button
        class="btn btn-danger btn-sm"
        type="button"
        :disabled="clearing || requestCount === 0"
        @click="$emit('confirm')"
      >
        {{ clearing ? 'Clearing…' : `Delete all ${requestCount}` }}
      </button>
    </div>
    <p v-if="error" class="failure">{{ error }}</p>
  </div>
</template>

<style scoped>
.confirm {
  display: flex;
  flex-direction: column;
  gap: 32px;
}
/* Right-aligned, the destructive action last. */
.actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 14px;
}
/* The failure is an error, so it is said in Brick. It sits inside the
   dialog, so it steps off the dialog's own fill rather than the page's,
   the way the dropdown's active option does. */
.failure {
  margin: 0;
  padding: 12px 14px;
  background: color-mix(in srgb, var(--surface-float) 92%, var(--text-body));
  border-radius: var(--radius-surface);
  font-size: var(--text-small);
  color: var(--danger);
}
</style>
