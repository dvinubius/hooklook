<script setup lang="ts">
/* An on/off switch: a square outlined track with a square knob, the outline
   turning to the accent while it is on. The label is part of the control — clicking the words
   flips it too — and screen readers hear a switch with its state. While
   `disabled`, it shows its state but cannot be flipped. */
const on = defineModel<boolean>({ required: true })
defineProps<{ label: string; disabled?: boolean }>()
</script>

<template>
  <label class="switch-field" :class="{ disabled }">
    <span class="text">{{ label }}</span>
    <button
      class="switch"
      :class="{ on }"
      type="button"
      role="switch"
      :aria-checked="on"
      :disabled="disabled"
      @click="on = !on"
    >
      <span class="knob" aria-hidden="true"></span>
    </button>
  </label>
</template>

<style scoped>
.switch-field {
  display: inline-flex;
  align-items: center;
  gap: 16px;
  cursor: pointer;
}
.text {
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  color: var(--text-dim);
}
.switch-field:not(.disabled):hover .text {
  color: var(--text-body);
}
.switch-field.disabled {
  cursor: not-allowed;
}
.switch:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.switch {
  position: relative;
  flex: none;
  width: 40px;
  height: 22px;
  padding: 0;
  border: 2px solid var(--text-muted);
  border-radius: var(--radius-control);
  background: transparent;
  cursor: pointer;
  transition: border-color 150ms ease-in-out;
}
/* Kept quiet: no fill in either state. Off, the outline is the muted grey;
   on, it is the accent. */
.switch.on {
  border-color: var(--accent);
}
/* Off, the knob matches the muted outline; on, it is the body text color —
   Paper on dark, Ink on light. It travels with the same ease as the colors. */
.knob {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 12px;
  height: 12px;
  border-radius: 2px;
  background: var(--text-muted);
  transition: transform 150ms ease-in-out, background-color 150ms ease-in-out;
}
.switch.on .knob {
  transform: translateX(18px);
  background: var(--text-body);
}
</style>
