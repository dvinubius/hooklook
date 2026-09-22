<script setup lang="ts">
/* How full the bin is, at the right end of the capture row.
   It sits with the controls that act on the bin rather than with the list,
   because it is a fact about the bin itself: the list can be filtered down to
   one row and the bin still be out of room.

   Two readings, because the bin has two limits and either one can be the
   binding one — a bin can be out of bytes with four hundred request slots to
   spare, and one number folding the two together would hide that. They sit
   side by side on the capture row's single line. The arithmetic is in
   `lib/capacity.ts`; this only says it. */
import { computed } from 'vue'
import {
  capacityAlarming,
  requestsDetail,
  requestsPercent,
  storageDetail,
  storagePercent,
} from '../lib/capacity'
import type { BinCapacity } from '../types'

const props = defineProps<{ capacity: BinCapacity }>()

const rows = computed(() => [
  {
    key: 'storage',
    label: 'storage',
    name: 'Bin storage used',
    percent: storagePercent(props.capacity),
    title: storageDetail(props.capacity),
  },
  {
    key: 'requests',
    label: 'requests',
    name: 'Bin request slots used',
    percent: requestsPercent(props.capacity),
    title: requestsDetail(props.capacity),
  },
])
</script>

<template>
  <div class="gauge code-surface">
    <template v-for="(row, index) in rows" :key="row.key">
      <span v-if="index > 0" class="separator" aria-hidden="true"></span>
      <span class="reading" :title="row.title">
        <span class="label">{{ row.label }}</span>
        <span
          class="track"
          :class="{ alarming: capacityAlarming(row.percent) }"
          role="progressbar"
          :aria-label="row.name"
          aria-valuemin="0"
          aria-valuemax="100"
          :aria-valuenow="row.percent"
        >
          <span class="fill" :style="{ width: `${row.percent}%` }"></span>
        </span>
        <span class="value" :class="{ alarming: capacityAlarming(row.percent) }">
          {{ row.percent }}%
        </span>
      </span>
    </template>
  </div>
</template>

<style scoped>
/* The two readings on one line, parted by the page's own upright hairline —
   the same one the header rows use between a label and its facts.

   On a code surface, and as high as the capture link beside it: the row is a
   set of blocks of one height, and a gauge floating on the page between two
   of them read as something the row had forgotten to frame. The surface's own
   13px is stepped down here — these are readings beside the link, not text
   level with it. */
.gauge {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  height: var(--row-height, 36px);
  padding: 0 12px;
  font-family: var(--font-mono);
  font-size: var(--text-mono-meta);
  color: var(--text-dim);
}
.reading {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.label {
  color: var(--text-muted);
}
/* Green while there is room, brick past the warning mark: two readings, one
   shape, no motion. The green is the one hue outside the brand palette, the
   same one the stream's live dot carries. The empty rail takes the page's
   hairline, so a bar reads on the page itself with no surface under it. */
.track {
  --gauge-ok: #2bac76;
  width: 56px;
  height: 6px;
  border-radius: 3px;
  background: var(--hairline);
  overflow: hidden;
}
.fill {
  display: block;
  height: 100%;
  background: var(--gauge-ok);
}
.value {
  font-variant-numeric: tabular-nums;
  text-align: right;
}
.track.alarming .fill {
  background: var(--danger);
}
.value.alarming {
  color: var(--danger);
}
</style>
