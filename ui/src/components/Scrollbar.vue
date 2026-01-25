<template>
  <div
    class="scrollbar"
    :class="{
      changed: isRecentChange,
      dragging: isDragging,
      scrolling: isScrolling,
      hovering: isHovering,
      precise: preciseAnchor !== null,
      finger: dragPointerType === 'touch' && thumbXOffset !== '0px',
    }"
    ref="container"
    @pointerdown="startDrag"
    @pointerenter="startHover"
    @pointerleave="stopHover"
  >
    <div
      class="track"
    >
    </div>
    <div
      class="markers"
    >
      <span
        v-for="marker in marker.items"
        :key="marker.y"
        class="marker"
        :style="{ top: marker.y + 'px' }"
      >
        {{ marker.label }}
      </span>
    </div>
    <div
      class="thumb"
      ref="thumb"
      :style="{ top: thumbTopPx + 'px' }"
    >
      <div class="tick"></div>
      <span class="marker">
        {{ thumbLabel }}
      </span>
    </div>
    <div
      v-if="isHovering && !isDragging"
      class="thumb"
      :style="{ top: thumbScrollPx + 'px', opacity: 0.3 }"
    >
      <div class="tick"></div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, toRefs, onUnmounted, watch } from 'vue';
import { defineProps } from 'vue';
import { useElementSize, watchDebounced } from '@vueuse/core';
import dateFormat from 'date-fns/format';

const props = defineProps({
  y: {
    type: Number,
    required: true
  },
  max: {
    type: Number,
    required: true
  },
  timestamps: {
    type: Int32Array,
  },
});

const {
  y,
  max,
  timestamps,
} = toRefs(props);

const emit = defineEmits({
  change: (y) => typeof y == "number",
});

const container = ref(null);
const thumb = ref(null);
const isDragging = ref(false);
const isScrolling = ref(false);
const isRecentChange = ref(false);
const isHovering = ref(false);
const preciseAnchor = ref(null);
const preciseDeadzone = 20;
const precisePixelThreshold = 200;
const preciseDelay = 1500;
const hoverY = ref(0);
const dragPointerType = ref("");
const thumbXOffset = ref("0px");

let preciseTentativeAnchor = 0;
let lastEventY = 0;

const containerSize = useElementSize(container);

watch(y, () => {
  isScrolling.value = true
});
watchDebounced(y, () => {
  isScrolling.value = false
}, { debounce: 2000 });

watch(timestamps, () => {
  isRecentChange.value = true
});
watchDebounced(timestamps, () => {
  isRecentChange.value = false
}, { debounce: 2000 });

const marker = computed(() => {
  if (!timestamps.value) return { level: 0, markers: [] };

  const date = new Date();
  const ts = timestamps.value;

  let year = -1;
  let month = -1;
  let day = -1;
  let hour = -1;
  let level = 4; // 4: hours, 3: days, 2: months, 1: years

  const maxCount = 3;
  let count = 0;

  const years = [];
  const months = [];
  const days = [];
  const hours = [];

  const minDist = 20;
  const maxDist = 100;
  const maxY = ts.length - minDist;

  for (let i = 0; i < maxY; i++) {
    const t = ts[i];
    date.setTime(t * 60000);
    const yr = date.getUTCFullYear();
    const mo = date.getUTCMonth();
    const dy = date.getUTCDate();
    const hr = date.getUTCHours();

    if (yr !== year && level >= 1) {
      if (year !== -1 && level > 1) {
        count++;
        if (count >= maxCount) {
          count = 0;
          level = 1;
        }
      }
      const minY = years.length > 0 ? years[years.length - 1].y + minDist : 0;
      if (minY > maxY) continue;
      if (minY - i < maxDist) years.push({ y: Math.max(minY, i), t, label: `${yr}` });
      year = yr;
    }
    if (mo !== month && level >= 2) {
      if (month !== -1 && level > 2) {
        count++;
        if (count >= maxCount) {
          count = 0;
          level = 2;
        }
      }
      const minY = months.length > 0 ? months[months.length - 1].y + minDist : 0;
      if (minY > maxY) continue;
      if (minY - i < maxDist) months.push({ y: Math.max(minY, i), t, label: dateFormat(date, "MMM yyyy") });
      month = mo;
    }
    if (dy !== day && level >= 3) {
      if (day !== -1 && level > 3) {
        count++;
        if (count >= maxCount) {
          count = 0;
          level = 3;
        }
      }
      const minY = days.length > 0 ? days[days.length - 1].y + minDist : 0;
      if (minY > maxY) continue;
      if (minY - i < maxDist) days.push({ y: Math.max(minY, i), t, label: dateFormat(date, "d MMM") });
      day = dy;
    }
    if (hr !== hour && level >= 4) {
      const minY = hours.length > 0 ? hours[hours.length - 1].y + minDist : 0;
      if (minY > maxY) continue;
      if (minY - i < maxDist) hours.push({ y: Math.max(minY, i), t, label: dateFormat(date, "HH:")+"00" });
      hour = hr;
    }
  }

  switch (level) {
    case 1:
      return { level, items: years };
    case 2:
      return { level, items: months };
    case 3:
      return { level, items: days };
    case 4:
      return { level, items: hours };
  }
  return { level, items: [] };
});

const thumbTickHeight = 4;
const thumbScrollPx = computed(() => {
  if (!containerSize.height.value) return 0;
  const ratio = y.value / max.value;
  const thumbMax = containerSize.height.value - thumbTickHeight;
  return ratio * thumbMax;
});

const thumbTopPx = computed(() => {
  if (isHovering.value && !isDragging.value) return hoverY.value;
  return thumbScrollPx.value;
});

const thumbLabel = computed(() => {
  if (!timestamps.value?.length) return "";
  if (!marker.value) return "";
  const ratio =
    isHovering.value && !isDragging.value ?
      hoverY.value / containerSize.height.value :
      y.value / max.value;
  if (isNaN(ratio)) return "";
  const index = Math.max(0, Math.min(timestamps.value.length - 1, Math.round(ratio * timestamps.value.length)));
  const t = timestamps.value[index];
  const date = new Date(t * 60000);
  if (isNaN(Number(date))) return "";

  let level = marker.value.level;
  const precise = preciseAnchor.value !== null;
  if (precise) level++;
  switch (level) {
    case 1:
      return dateFormat(date, "d MMM yyyy");
    case 2:
      return dateFormat(date, "d MMM yyyy");
    case 3:
      return dateFormat(date, "d MMM HH:mm");
    case 4:
      return dateFormat(date, "HH:mm");
  }
  return "";
});

const startDrag = (event) => {
  isDragging.value = true;
  preciseTentativeAnchor = -preciseDeadzone;
  document.addEventListener('pointermove', handleDrag);
  document.addEventListener('pointerup', stopDrag);
  handleDrag(event);
};

const stopDrag = (event) => {
  isDragging.value = false;
  thumbXOffset.value = "0px";
  disablePrecision();
  document.removeEventListener('pointermove', handleDrag);
  document.removeEventListener('pointerup', stopDrag);
  if (isHovering.value) {
    handleHover(event);
  }
};

const startHover = (event) => {
  document.addEventListener('pointermove', handleHover);
  document.addEventListener('pointerleave', stopHover);
  isHovering.value = true;
  handleHover(event);
};

const stopHover = () => {
  if (!isHovering.value) return;
  document.removeEventListener('pointermove', handleHover);
  document.removeEventListener('pointerleave', stopHover);
  isHovering.value = false;
};

let precisionTimeout = null;

const enablePrecision = () => {
  preciseAnchor.value = lastEventY;
};

const disablePrecision = () => {
  preciseAnchor.value = null;
  clearTimeout(precisionTimeout);
  precisionTimeout = null;
};

const handleDrag = (event) => {
  if (!isDragging.value) return;
  const containerRect = container.value.getBoundingClientRect();
  const y = event.clientY - containerRect.top;
  const h = containerRect.height;
  dragPointerType.value = event.pointerType;
  
  let newRatio = 0;
  
  const xDiff = event.clientX - containerRect.left;
  if (event.clientX < containerRect.left * 0.5) {
    thumbXOffset.value = "0px";
  } else {
    const tabStop = 100;
    thumbXOffset.value = ((Math.round((-xDiff + 50)/tabStop) + 0.5)*tabStop) + "px";
  }

  if (preciseAnchor.value === null) {
    newRatio = y / h;

    const pixelHeight = max.value / h;
    if (pixelHeight > precisePixelThreshold) {
      const tentativeAnchorDiff = y - preciseTentativeAnchor;
      if (Math.abs(tentativeAnchorDiff) > preciseDeadzone) {
        preciseTentativeAnchor = y;
        disablePrecision();
        precisionTimeout = setTimeout(enablePrecision, preciseDelay);
      }
    }
  } else {
    const yDiff = y - preciseAnchor.value;
    const speed = 0.1;
    newRatio = (preciseAnchor.value + yDiff * speed) / h;
  }

  lastEventY = y;

  newRatio = Math.max(0, Math.min(1, newRatio));

  const newY = newRatio * max.value;
  emit('change', newY);
};

const handleHover = (event) => {
  if (isDragging.value) return;
  const containerRect = container.value.getBoundingClientRect();
  const y = event.clientY - containerRect.top;
  hoverY.value = y;
};

onUnmounted(() => {
  stopDrag();
  stopHover();
});

</script>

<style scoped>
.scrollbar {
  width: 60px;
  user-select: none;
  touch-action: none;
}

.track {
  position: absolute;
  right: 0;
  width: 20px;
  height: 100%;
}

@media (max-width: 700px) {
  .scrollbar {
    pointer-events: none;
  }

  .thumb {
    opacity: 0;
  }

  .scrollbar.scrolling .thumb, .scrollbar.dragging .thumb {
    opacity: 1;
    pointer-events: visible;
  }
}

.scrollbar {
  --thumb-width: 18px;
  cursor: grab;
}

.scrollbar.dragging {
  cursor: grabbing;
}

.scrollbar.dragging .thumb {
  --thumb-width: 40px;
}

.scrollbar.precise .thumb {
  --thumb-width: 60px;
}

.thumb {
  position: absolute;
  --thumb-hitbox: 60px;
  margin-top: calc(var(--thumb-hitbox) / -2);
  right: 0;
  width: var(--thumb-hitbox);
  height: var(--thumb-hitbox);
  transition: opacity 0.4s;
}

.thumb .tick {
  position: absolute;
  --tick-height: 2px;
  top: calc(50% - var(--tick-height) / 2);
  right: 0;
  width: var(--thumb-width);
  height: var(--tick-height);
  background-color: var(--mdc-theme-primary);
  border: 2px solid var(--mdc-theme-background);
  border-right: none;
  transition: width 0.1s;
}

.thumb .marker {
  opacity: 0;
  transition: opacity 3s cubic-bezier(0.895, 0.03, 0.685, 0.22), right 0.4s, bottom 0.4s;
}


.scrollbar.hovering .thumb .marker, .scrollbar.dragging .thumb .marker {
  opacity: 1;
  transition: right 1s, bottom 0.4s;
}

.markers {
  opacity: 0;
  transition: opacity 0.5s;
}

.scrollbar.hovering .markers, .scrollbar.scrolling .markers, .scrollbar.dragging .markers, .scrollbar.changed .markers {
  opacity: 1;
}

.marker {
  position: absolute;
  right: 0;
  text-align: right;
  text-wrap: nowrap;
  padding: 2px 6px 2px 6px;
  font-size: 0.8em;
  color: var(--mdc-theme-text-secondary-on-background);
  background-color: var(--mdc-theme-background);
  border-radius: 5px 0 0 5px;
  transition: top 0.4s;
}

.thumb .marker {
  bottom: calc(50%);
  border-bottom-left-radius: 0;
}

.scrollbar.finger .thumb .marker {
  bottom: calc(50% - 0.6em);
  right: calc(var(--thumb-width) + v-bind(thumbXOffset) + 4px);
  border-radius: 5px;
}


</style>