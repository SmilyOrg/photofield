<template>
  <div
    class="scrollbar"
    :class="{
      'scene-change': isRecentSceneChange,
      dragging: isDragging,
      scrolling: isScrolling,
      precise: preciseAnchor !== null,
      finger: dragPointerType === 'touch' && thumbXOffset !== '0px',
    }"
    ref="container"
    @pointerdown="startDrag"
    @pointerenter="startHover"
    @pointerleave="stopHover"
  >
    <!-- {{ isRecentSceneChange }} -->
    <div
      class="track"
    >
      <!-- <img :src="minimapTileUrl" draggable="false" /> -->
    </div>
    <div
      class="markers"
    >
      <span
        v-for="marker in marker.items"
        :key="marker.label"
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
      <span class="marker">
        {{ thumbLabel }}
      </span>
    </div>
    <div
      v-if="isHovering && !isDragging"
      class="thumb"
      :style="{ top: thumbScrollPx + 'px', opacity: 0.3 }"
    >
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, watch, computed, toRefs, onUnmounted } from 'vue';
import { defineProps } from 'vue';
import { getTileUrl } from '../api';
import { useTimeline } from '../use';
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
  scene: {
    type: Object,
  },
});

const {
  y,
  max,
  scene,
} = toRefs(props);

const emit = defineEmits({
  change: (y) => typeof y == "number",
});

const container = ref(null);
const thumb = ref(null);
const isDragging = ref(false);
const isScrolling = ref(false);
const isRecentSceneChange = ref(false);
const isHovering = ref(false);
const preciseAnchor = ref(null);
// const preciseSpeed = ref(1);
const preciseDeadzone = 20;
const precisePixelThreshold = 200;
const preciseDelay = 1500;
const hoverY = ref(0);

const active = computed(() => {
  return isDragging.value || isHovering.value;
})

// const containerSize = computed(() => {
//   if (!container.value) return { width: 0, height: 0 };
//   const rect = container.value.getBoundingClientRect();
//   return { width: rect.width, height: rect.height };
// });

const containerSize = useElementSize(container);

const { timestamps } = useTimeline({
  scene,
  height: containerSize.height,
});

watchDebounced(y, () => {
  isScrolling.value = false;
}, {
  debounce: 2000,
  onTrigger() {
    isScrolling.value = true;
  }
});

watchDebounced(scene, () => {
  isRecentSceneChange.value = false;
}, {
  debounce: 2000,
  onTrigger() {
    isRecentSceneChange.value = true;
  }
});

function timezoneOffsetNow() {
  const date = new Date();
  return date.getTimezoneOffset() * 60;
}

const marker = computed(() => {
  if (!timestamps.value) return { level: 0, markers: [] };
  const markers = [];

  const offset = timezoneOffsetNow();
  const date = new Date();
  const ts = timestamps.value;

  let year = -1;
  let month = -1;
  let day = -1;
  let level = 3; // 3: days, 2: months, 1: years

  const years = [];
  const months = [];
  const days = [];

  const minDist = 20;

  for (let i = 0; i < ts.length - minDist; i++) {
    const t = ts[i];
    date.setTime(t * 1000 + offset);
    const yr = date.getFullYear();
    const mo = date.getMonth();
    const dy = date.getDate();

    // console.log(i, yr, mo, dy);

    if (yr !== year && level >= 1) {
      if (year !== -1 && level > 1) {
        level = 1;
      }
      // const dist = years.length == 0 ? Infinity : Math.abs(years[years.length - 1].y - i);
      const minY = years.length > 0 ? years[years.length - 1].y + minDist : 0;
      // console.log(i, minY, years.length > 0 ? years[years.length - 1].y : 0);
      years.push({ y: Math.max(minY, i), label: `${yr}` });
      year = yr;
    }
    if (mo !== month && level >= 2) {
      if (month !== -1 && level > 2) {
        level = 2;
      }
      const dist = months.length == 0 ? Infinity : Math.abs(months[months.length - 1].y - i);
      if (dist > minDist) months.push({ y: i, label: dateFormat(date, "MMM yyyy") });
      month = mo;
    }
    if (dy !== day && level >= 3) {
      const dist = days.length == 0 ? Infinity : Math.abs(days[days.length - 1].y - i);
      if (dist > minDist) days.push({ y: i, label: dateFormat(date, "d MMM") });
      day = dy;
    }
  }

  switch (level) {
    case 1:
      return { level, items: years };
    case 2:
      return { level, items: months };
    case 3:
      return { level, items: days };
  }
  return { level, items: [] };
});

// watch(markers, (newValue) => {
//   console.log(newValue);
// });


const thumbScrollPx = computed(() => {
  if (!containerSize.height.value || !thumb.value) return 0;
  const ratio = y.value / max.value;
  const thumbMax = containerSize.height.value - thumb.value.offsetHeight;
  return ratio * thumbMax;
});

const thumbTopPx = computed(() => {
  if (isHovering.value && !isDragging.value) return hoverY.value;
  return thumbScrollPx.value;
});

const thumbLabel = computed(() => {
  if (!timestamps.value?.length) return "";
  if (!marker.value) return "";
  const offset = timezoneOffsetNow();
  const ratio =
    isHovering.value && !isDragging.value ?
      hoverY.value / containerSize.height.value :
      y.value / max.value;
  const index = Math.max(0, Math.min(timestamps.value.length - 1, Math.round(ratio * timestamps.value.length)));
  const t = timestamps.value[index];
  // console.log("timestamp", t, timestamps.value.length, index);
  const date = new Date(t * 1000 + offset);
  let level = marker.value.level;
  const precise = preciseAnchor.value !== null;
  if (precise) level++;
  switch (level) {
    case 1:
      return dateFormat(date, "MMM yyyy");
    case 2:
      return dateFormat(date, "d MMM");
    case 3:
      return dateFormat(date, "EEE HH:mm");
    case 4:
      return dateFormat(date, "HH:mm:ss");
  }
  return "";
});

const resolution = 6;

// const minimapTileSize = computed(() => {
//   if (!containerSize.height.value) return null;
//   return Math.round(containerSize.height.value * resolution);
// });

// const minimapTileUrl = computed(() => {
//   if (!scene.value?.id) return null;
//   return getTileUrl(
//     scene.value.id,
//     0, 0, 0,
//     minimapTileSize.value,
//     null,
//     // { transparency_mask: true }
//   );
// });

// const minimapWidth = computed(() => {
//   const bounds = scene.value?.bounds;
//   if (!bounds) return 0;
//   return bounds.w / bounds.h * containerSize.value.height;
// });

// const minimapStretch = computed(() => {
//   return minimapTileSize.value / resolution / minimapWidth.value * 100 + "%";
// });

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
  // preciseSpeed.value = 1;
};

const disablePrecision = () => {
  preciseAnchor.value = null;
  clearTimeout(precisionTimeout);
  precisionTimeout = null;
  // preciseSpeed.value = 1;
};

let preciseTentativeAnchor = 0;
let lastEventY = 0;
const dragPointerType = ref("");
const thumbXOffset = ref("0px");

const handleDrag = (event) => {
  if (!isDragging.value) return;
  const containerRect = container.value.getBoundingClientRect();
  // let newTop = event.clientY - containerRect.top;
  // newTop = Math.max(0, Math.min(newTop, containerRect.height - thumb.value.offsetHeight));
  const y = event.clientY - containerRect.top;
  const h = containerRect.height;
  dragPointerType.value = event.pointerType;
  
  // const xDiff = event.clientX - containerRect.left;
  // const xDiff = event.clientX - containerRect.left * 0.5;
  let newRatio = 0;
  
  const xDiff = event.clientX - containerRect.left;
  if (event.clientX < containerRect.left * 0.5) {
    thumbXOffset.value = "0px";
  } else {
    // thumbXOffset.value = (-xDiff + 70) + "px";
    const tabStop = 100;
    // const tabStop = 1;
    // thumbXOffset.value = (10 + Math.round((-xDiff + 50)/tabStop)*tabStop) + "px";
    thumbXOffset.value = ((Math.round((-xDiff + 50)/tabStop) + 0.5)*tabStop) + "px";
    // thumbXOffset.value = (-xDiff + 80) + "px";
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
    // disablePrecision();
    // precisionTimeout = setTimeout(enablePrecision.bind(null, y), 1000);
  } else {
    const yDiff = y - preciseAnchor.value;
    // const speed = 1 / (1 - xDiff * 0.03);
    const speed = 0.1;
    // preciseSpeed.value = speed;
    newRatio = (preciseAnchor.value + yDiff * speed) / h;
    // console.log("preciseAnchor", preciseAnchor.value, "yDiff", yDiff, "speed", speed);
  }

  lastEventY = y;


  // if (xDiff < 0) {
  //   if (preciseAnchor.value === null) {
  //     preciseAnchor.value = y;
  //   }
  //   const yDiff = y - preciseAnchor.value;
  //   const speed = 1 / (1 - xDiff * 0.03);
  //   preciseSpeed.value = speed;
  //   console.log("xDiff", xDiff, "yDiff", yDiff, "speed", speed);
  //   newRatio = (preciseAnchor.value + yDiff * speed) / h;
  // } else {
  //   preciseAnchor.value = null;
  //   newRatio = y / h;
  // }
  
  
  // console.log(newRatio);

  newRatio = Math.max(0, Math.min(1, newRatio));

  const newY = newRatio * max.value;
  emit('change', newY);

  // event.stopPropagation();
  // event.stopImmediatePropagation();
  // console.log(newY);
  // thumb.value.style.top = `${newTop}px`;
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

// watch(() => props.y, (newY) => {
//   const containerRect = container.value.getBoundingClientRect();
//   let newTop = (newY / props.max) * containerRect.height;
//   newTop = Math.max(0, Math.min(newTop, containerRect.height - thumb.value.offsetHeight));
//   thumb.value.style.top = `${newTop}px`;
// });
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
  /* background-color: #dddddd; */
}

/* .track img { */
  /* clip-path: xywh(0, 0, v-bind(minimapWidth), 100%); */
  /* width: v-bind(minimapStretch); */
  /* pixelart style */
  /* image-rendering: pixelated; */
  /* faded to white */
  /* filter: contrast(0.5) brightness(1.5); */
  /* width: 3000px; */
/* } */

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
  right: 0;
  width: var(--thumb-width);
  height: 2px;
  background-color: #6782ff;
  border: 2px solid white;
  border-right: none;
  transition: width 0.1s;
}

.thumb .marker {
  opacity: 0;
  transition: opacity 3s cubic-bezier(0.895, 0.03, 0.685, 0.22), right 0.4s, bottom 0.4s;
  /* transition: opacity 1s; */
}


.scrollbar:hover .thumb .marker, .scrollbar.dragging .thumb .marker {
  opacity: 1;
  transition: right 1s, bottom 0.4s;
  /* transition: none; */
}

.markers {
  opacity: 0;
  transition: opacity 0.5s;
}

.scrollbar:hover .markers, .scrollbar.scrolling .markers, .scrollbar.dragging .markers, .scrollbar.scene-change .markers {
  opacity: 1;
}

.marker {
  position: absolute;
  right: 0;
  text-align: right;
  text-wrap: nowrap;
  padding: 2px 6px 2px 6px;
  font-size: 0.8em;
  color: #666;
  background-color: rgba(255, 255, 255, 1);
  border-radius: 5px 0 0 5px;
  transition: top 0.4s;
}

.thumb .marker {
  bottom: 4px;
  border-bottom-left-radius: 0;
}

/* .scrollbar.dragging .thumb {
  width: 40px;
}

.scrollbar.precise .thumb {
  width: 60px;
} */

.scrollbar.finger .thumb .marker {
  /* bottom: 40px; */
  bottom: -0.6em;
  /* right: calc(var(--thumb-width) + 4px); */
  right: calc(var(--thumb-width) + v-bind(thumbXOffset) + 4px);
  border-radius: 5px;
}


</style>