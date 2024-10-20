<template>
  <div class="scroll" :class="{ fullpage, fixed: false }">

    <tile-viewer
      class="viewer"
      ref="viewer"
      :style="{ transform: `translate(0, ${nativeScrollY}px)` }"
      :scene="scene"
      :view="view"
      :selectTag="selectTag"
      :debug="debug"
      :tileSize="512"
      :interactive="interactive"
      :pannable="!nativeScroll && interactive"
      :zoomable="!nativeScroll && interactive"
      :zoom-transition="regionTransition"
      :focus="!!region"
      :crossNav="!!region"
      :viewport="viewport"
      :qualityPreset="qualityPreset"
      @click="onClick"
      @view="onView"
      @nav="onNav"
      @wheel="onWheel"
      @load-end="onLoadEnd"
      @contextmenu.prevent="onContextMenu"
      @keydown.esc="onEscape"
      @box-select="onBoxSelect"
      @viewer="emit('viewer', $event)"
    ></tile-viewer>

    <!-- <div
      class="viewer"
      ref="viewer"
    ></div> -->
    <!-- <pre
      :style="{
        position: 'absolute',
        top: '0',
        left: '0',
        height: '0',
        overflow: 'visible',
      }"
    >
      <template v-for="i in 100">
        <div
          v-if="i % 1 === 0"
          :style="{
            position: 'absolute',
            top: `${i*100}px`,
            left: 0,
            width: i % 10 === 0 ? '200px' : '50px',
            height: '1px',
            backgroundColor: 'black',
          }"
        ></div>
        <div
          v-if="i % 1 === 0"
          :style="{
            position: 'absolute',
            top: `${i*100 - 25}px`,
            left: '225px',
            fontSize: i % 10 === 0 ? '40px' : '20px',
          }"
        >{{ i*100 }}</div>
      </template>
    </pre> -->

    <Spinner
      class="spinner"
      :total="scene?.file_count"
      :speed="loadSpeed"
      :divider="10000"
      :loading="scene?.loading"
    ></Spinner>

    <DateStrip
      class="date-strip"
      :class="{ visible: scrollSpeed > viewport.height.value * 8 }"
      :date="scrollDate"
    ></DateStrip>

    <div
      class="virtual-canvas"
      :style="{ height: nativeScrollHeight + 'px' }">
    </div>
      
    <!-- <div
      class="scroller"
      ref="scroller"
    > -->
      <!-- <div
        class="virtual-canvas"
        :style="{ height: nativeScrollHeight + 'px' }">
      </div> -->
      <!-- :style="{ height: canvas.height + 'px' }"> -->
    <!-- </div> -->

    <!-- <RectDebug
      v-if="viewport.width"
      :style="{ position: 'fixed', top: '0', left: '50%', zIndex: 1000, transform: 'translate(-50%, 0)' }"
      :rectangles="debugRects"
      :width="canvas.width"
      :height="canvas.height"
      :drawWidth="40"
      :drawHeight="600"
    ></RectDebug> -->

    <Scrollbar
      v-if="!region"
      class="scrollbar"
      :y="scrollY"
      :max="scrollMax"
      :scene="scene"
      @change="scrollToPixels"
    ></Scrollbar>

    <ContextMenu
      class="context-menu"
      ref="contextMenu"
    >
      <RegionMenu
        :scene="scene"
        :region="contextRegion"
        :flipX="contextFlip.x"
        :flipY="contextFlip.y"
        :tileSize="512"
        @close="closeContextMenu()"
        @search="emit('search', $event)"
      ></RegionMenu>
    </ContextMenu>
  </div>
</template>

<script setup>
import ContextMenu from '@overcoder/vue-context-menu';
import { useEventBus, useScroll, useWindowScroll, useWindowSize, watchDebounced } from '@vueuse/core';
import { computed, nextTick, onMounted, onUnmounted, ref, toRefs, watch } from 'vue';
import { getRegion, getRegions, useScene, useApi, getRegionClosestTo } from '../api';
import { useSeekableRegion, useScrollbar, useViewport, useContextMenu, useTimeline, useTags, useTimelineDate } from '../use.js';
import DateStrip from './DateStrip.vue';
import RegionMenu from './RegionMenu.vue';
import Spinner from './Spinner.vue';
import TileViewer from './TileViewer.vue';
import RectDebug from './RectDebug.vue';
import Scrollbar from './Scrollbar.vue';

const props = defineProps({
  interactive: Boolean,
  collectionId: String,
  regionId: String,
  focusFileId: String,
  layout: String,
  sort: String,
  imageHeight: Number,
  search: String,
  selectTag: Object,
  debug: Object,
  fullpage: Boolean,
  scrollbar: Object,
  tweaks: String,
});

const emit = defineEmits({
  loadEnd: null,
  tasks: null,
  immersive: immersive => typeof immersive == "boolean",
  scene: null,
  reindex: null,
  region: null,
  selectTag: null,
  search: null,
  elementView: null,
  viewer: null,
  swipeUp: null,
  focusFileId: null,
})

const {
  interactive,
  regionId,
  focusFileId,
  collectionId,
  scrollbar,
  layout,
  sort,
  imageHeight,
  search,
  selectTag,
  debug,
  tweaks,
} = toRefs(props);

const viewer = ref(null);
const viewport = useViewport(viewer);

const lastView = ref(null);
const lastNonNativeView = ref(null);
let lastLoadedScene = null;
let lastFocusFileId = null;

const windowSize = useWindowSize();

// const nativeScrollHeight = ref(10000);
// const nativeScrollHeight = ref(1000000);
const nativeScrollHeight = computed(() => {
  // return Math.min(1000000, canvas.value.height - windowSize.height.value);
  // return Math.min(1000000, scene.value?.bounds.h - windowSize.height.value);
  // return Math.min(1000000, scene.value?.bounds.h - viewport.height.value);
  return Math.min(100000, scene.value?.bounds.h - viewport.height.value);
  // return Math.min(10000, canvas.value.height);
});
// const nativeScrollMax = computed(() => {
//   // return Math.min(1000000, canvas.value.height - viewport.height.value);
// });
// const nativeScrollHeight = computed(() => {
//   return canvas.value.height;
// });
// const scrollOffset = ref(0);
let scrollOffset = 0;
const scrollY = ref(0);

const focusScreenRatioY = 0.33;

const { scene, recreate: recreateScene, loadSpeed } = useScene({
  layout,
  sort,
  collectionId,
  imageHeight,
  viewport,
  search,
  tweaks,
});

const qualityPreset = computed(() => {
  if (tweaks.value?.indexOf("hq") > -1) return "HIGH";
  return null;
});

watch(scene, async (newScene) => {
  if (!newScene || newScene.loading) return;
  if (newScene?.id == lastLoadedScene?.id && newScene?.loading == lastLoadedScene?.loading) {
    return;
  }
  if (lastLoadedScene && newScene.search != lastLoadedScene.search) {
    emit("focusFileId", null);
    scrollToPixels(0);
  }
  lastLoadedScene = newScene;
  emit("scene", newScene);
});

const {
  items: focusRegions,
} = useApi(() =>
  scene.value && focusFileId.value && focusFileId.value != lastFocusFileId &&
  `/scenes/${scene.value?.id}/regions?file_id=${focusFileId.value}`
);

const focusRegion = computed(() => {
  if (!focusFileId.value) return null;
  if (focusFileId.value == lastFocusFileId) return null;
  return focusRegions.value?.[0];
});

useEventBus("recreate-scene").on(scene => {
  if (scene?.name && scene?.name != "Scroll") return;
  recreateScene();
});

const {
  region,
  navigate,
  exit: regionExit,
} = useSeekableRegion({
  scene,
  collectionId,
  regionId,
})

const regionTransition = ref(false);
watch(region, async (newRegion, oldRegion) => {
  regionTransition.value = !!((!newRegion && oldRegion) || (newRegion && !oldRegion));
  emit("region", newRegion);
}, { immediate: true });

const exit = async () => {
  await centerToBounds(lastNonNativeView.value);
  await regionExit();
}

const { data: capabilities } = useApi(() => "/capabilities");
const tagsSupported = computed(() => capabilities.value?.tags?.supported);

const contextMenu = ref(null);
const {
  onContextMenu,
  flip: contextFlip,
  close: closeContextMenu,
  region: contextRegion,
} = useContextMenu(contextMenu, viewer, scene);

const canvas = computed(() => {
  if (
    !viewport.width.value ||
    !viewport.height.value ||
    !scene.value?.bounds?.w ||
    !scene.value?.bounds?.h
  ) {
    return { width: 1, height: 1 }
  }
  const aspectRatio = scene.value.bounds.w / scene.value.bounds.h;
  return {
    width: viewport.width.value,
    height: viewport.width.value / aspectRatio,
  }
});

const lastZoom = computed(() => {
  return scene.value?.bounds.w / lastView.value?.w;
});

const nativeScroll = computed(() => {
  if (lastZoom.value > 1.2) {
    return false;
  }

  if (region.value) return false;
  return true;
});

// watch(nativeScroll, async (newValue, oldValue) => {
//   if (newValue == oldValue) {
//     return;
//   }
//   if (newValue) {
//     await centerToBounds(lastNonNativeView.value);
//   }
// });


const centerToBounds = async (bounds) => {
  const by = bounds.y + bounds.h * 0.5;
  const vy = viewport.height.value * 0.5;
  await nextTick();
  scrollToPixels(by - vy);
  await nextTick();
}

const onEscape = async () => {
  if (selectTag.value) {
    emit("selectTag", null);
    return;
  }
  zoomOut();
  if (lastView.value) {
    const lastZoom = scene.value.bounds.w / lastView.value.w;
    if (lastZoom > 1.1) {
      return;
    }
  }
}

const zoomOut = () => {
  viewer.value?.setView(view.value);
}

const scrollSleep = computed(() => {
  return !nativeScroll.value || lastZoom.value > 1.0001;
});

// const {
//   y: scrollY,
//   yPerSec: scrollSpeed,
//   ratio: scrollRatio,
//   max: scrollMax,
//   scrollToPixels,
// } = useScrollbar(scrollbar, scrollSleep);

// const nativeScrollY = computed(() => {
//   return window.scrollY;
// });

const nativeScrollY = ref(window.scrollY);
// const nativeScrollMax = ref(window.scrollMaxY);
function nativeScrollTo(y) {
  window.scrollTo(0, y);
  nativeScrollY.value = y;
}

function scrollToPixels(y) {
  const nativeHeight = nativeScrollHeight.value;
  const maxOffset = scrollMax.value - nativeHeight + viewport.height.value;
  const nativeScrollTarget = nativeHeight * 0.5;
  const ty = y - nativeScrollTarget;
  if (ty < 0) {
    scrollOffset = 0;
    nativeScrollTo(y);
  } else if (ty > maxOffset) {
    scrollOffset = maxOffset;
    nativeScrollTo(y - maxOffset);
  } else {
    scrollOffset = ty;
    if (nativeScrollY.value != nativeScrollTarget) {
      nativeScrollTo(nativeScrollTarget);
    }
  }
  scrollY.value = nativeScrollY.value + scrollOffset;
  // console.log("scrollToPixels", Math.round(y), "nativeScrollY", Math.round(nativeScrollY.value), "scrollOffset", Math.round(scrollOffset), "scrollY", Math.round(scrollY.value));
}

function updateScrollFromNative(y) {
  const actionDistanceRatio = 0.1;
  const nativeHeight = nativeScrollHeight.value;
  const maxOffset = scrollMax.value - nativeHeight + viewport.height.value;
  const actionDist = nativeHeight * actionDistanceRatio;
  const nativeScrollTarget = nativeHeight * 0.5;
  const diff = nativeScrollTarget - nativeScrollY.value;
  const ty = y - nativeScrollTarget;
  if (ty < 0) {
    scrollOffset = 0;
  } else if (ty > maxOffset) {
    scrollOffset = maxOffset;
  } else if (Math.abs(diff) > actionDist) {
    scrollOffset = ty;
    nativeScrollTo(nativeScrollTarget);
  }
  scrollY.value = nativeScrollY.value + scrollOffset;
}

function onWindowScroll() {
  nativeScrollY.value = window.scrollY;
}
// function onWindowResize() {
//   nativeScrollMax.value = window.scrollMaxY;
// }
onMounted(() => {
  window.addEventListener("scroll", onWindowScroll);
  document.documentElement.classList.add("hide-scrollbar");
  // window.addEventListener("resize", onWindowResize);
});
onUnmounted(() => {
  window.removeEventListener("scroll", onWindowScroll);
  document.documentElement.classList.remove("hide-scrollbar");
  // window.removeEventListener("resize", onWindowResize);
});

const scrollSpeed = ref(0);
// const scrollFrameDiff = ref(0);

const scrollMax = computed(() => {
  return canvas.value.height - viewport.height.value;
});

const scrollRatio = computed(() => {
  return scrollY.value / scrollMax.value;
});

watch(scrollRatio, (ratio) => {
  // console.log("scrollY", scrollY.value, "scrollMax", scrollMax.value, "nativeScrollY", nativeScrollY.value, "nativeScrollHeight", nativeScrollHeight.value, "scrollOffset", scrollOffset, "canvas", canvas.value.height, "viewport", viewport.height.value, "scrollRatio", ratio);
  // console.log("scrollMax", scrollMax.value, "nativeScrollHeight", nativeScrollHeight.value, "canvas", canvas.value.height, "viewport", viewport.height.value, "window", window.scrollMaxY);
  // console.log("viewport", viewport.height.value, "window", window.scrollMaxY, "scene", scene.value?.bounds.h);
});


let lastScrollTime = 0;
let scrollSpeedResetTimer = null;
watch(scrollY, (y, oldy) => {
  const now = Date.now();
  const dt = now - lastScrollTime;
  lastScrollTime = now;
  // scrollFrameDiff.value = y - oldy;
  // requestAnimationFrame(resetScrollFrameDiff);
  if (dt == 0 || dt > 200) {
    return;
  }
  scrollSpeed.value = Math.abs(y - oldy) * 1000 / dt;
  clearTimeout(scrollSpeedResetTimer);
  scrollSpeedResetTimer = setTimeout(resetScrollSpeed, 100);
  // console.log("scrollSpeed", scrollSpeed.value);
  // if (lastScrollTime) {

  // }
});

function resetScrollSpeed() {
  scrollSpeed.value = 0;
}

function resetScrollFrameDiff() {
  requestAnimationFrame(resetScrollFrameDiff2);
}

function resetScrollFrameDiff2() {
  scrollFrameDiff.value = 0;
}

watch(nativeScrollY, () => {
  if (!nativeScroll.value) return;
  if (!canvas.value.height) return;
  if (!viewport.height.value) return;

  // const nativeHeight = nativeScrollHeight.value;
  // const nsy = nativeScrollY.value;
  // const max = scrollMax.value - nativeHeight + viewport.height.value;
  // const actionDistanceRatio = 0.2;


  // console.log("nativeScrollY", nativeScrollY.value);

  // const actionDistanceRatio = 0;
  
  // const half = scrollMax.value * 0.5;
  // console.log("canvas", Math.round(canvas.value.height), "viewport", Math.round(viewport.height.value), "scrollMax", Math.round(scrollMax.value), "nativeScrollHeight", Math.round(nativeScrollHeight.value), "nativeScrollY", Math.round(nativeScrollY.value), "scrollOffset", Math.round(scrollOffset.value), "scrollY", Math.round(scrollY.value), "sy", Math.round(sy));

  // const nativeScrollTarget = nativeHeight * 0.5;
  // let cy = scrollOffset + nsy - nativeScrollTarget;
  // if (cy < 0) {
  //   cy = 0;
  // } else if (cy > max) {
  //   cy = max;
  // } else {
  //   scrollToPixels(half);
  // }
  // scrollOffset.value = cy;

  // const nativeScrollTarget = nativeHeight * 0.5;
  // const actionDist = nativeHeight * actionDistanceRatio;
  // const diff = nativeScrollTarget - nsy;
  // let ty = scrollOffset + nsy - nativeScrollTarget;
  // let ty = scrollOffset + nativeScrollY.value;
  updateScrollFromNative(scrollOffset + nativeScrollY.value);
  // if (ty < 0) {
  //   scrollOffset = 0;
  //   ty = 0;
  // } else if (ty > max) {
  //   scrollOffset = max;
  //   ty = max;
  // } else if (Math.abs(diff) > actionDist) {
  //   scrollOffset = ty;
  //   nativeScrollTo(nativeScrollTarget);
  //   // scrollToPixels(nativeScrollTarget);
  // }
  // console.log("nativeScrollY", nsy, "scrollOffset", scrollOffset, "ty", ty);

  // if (Math.abs(diff) > actionDist) {
  //   scrollOffset -= diff;
  //   nativeScrollTo(nativeScrollTarget);
  // }
  
  // let cy = scrollOffset.value + nativeScrollY.value - half;
  // if (cy < 0) {
  //   cy = 0;
  // } else if (cy > max) {
  //   cy = max;
  // } else {
  //   scrollToPixels(half);
  // }
  // scrollOffset.value = cy;
});


// watchDebounced(scrollY, async (sy) => {
//   if (!scene.value) return;
//   if (!view.value || !view.value.w || !view.value.h) return;
//   if (sy < 500) {
//     if (!lastFocusFileId) return;
//     lastFocusFileId = null;
//     emit("focusFileId", null);
//     return;
//   }
//   const { x, y, w, h } = view.value;
//   const center = await getRegionClosestTo(
//     scene.value.id,
//     x, y + h * focusScreenRatioY,
//   );
//   const fileId = center?.data?.id;
//   if (!fileId) return;
//   lastFocusFileId = fileId;
//   emit("focusFileId", fileId);
// }, { debounce: 1000 });

const { date: scrollDate } = useTimelineDate({ scene, viewport, scrollRatio });

const maxScrollY = computed(() => {
  return Math.max(1, canvas.value.height - viewport.height.value);
});

const view = computed(() => {
  if (region.value) {
    return region.value.bounds;
  }

  // const sy = Math.min(scrollY.value, maxScrollY.value - 1);
  const sy = scrollY.value;

  return {
    x: 0,
    y: sy,
    w: viewport.width.value,
    h: viewport.height.value,
  }
});

// watch([focusRegion, scrollMax], async ([focusRegion, _]) => {
//   if (!focusRegion) return;
//   if (canvas.height <= 1) return;
//   if (regionId.value) return;
//   const bounds = focusRegion.bounds;
//   scrollToPixels(bounds.y + bounds.h * 0.5 - viewport.height.value * focusScreenRatioY);
// });

const {
  selectBounds
} = useTags({
  supported: tagsSupported,
  selectTag,
  collectionId,
  scene,
});

const onClick = async (event) => {
  if (!event) return false;
  if (region.value) return false;
  if (tagsSupported.value && (selectTag.value || event.originalEvent.ctrlKey)) {
    const tag = await selectBounds("INVERT", {
      x: event.x,
      y: event.y,
      w: 0,
      h: 0,
    });
    emit("selectTag", tag);
    return false;
  }
  const regions = await getRegions(scene.value?.id, event.x, event.y, 0, 0);
  if (regions && regions.length > 0) {
    if (regions[0].id == region.value?.id) {
      return false;
    }
    emit("region", regions[0]);
    return true;
  }
  return false;
}

const onWheel = async (event) => {
  if (event.ctrlKey && nativeScroll.value) {
    event.preventDefault();
    if (event.deltaY < 0) {
      const bump = 0.3;
      // Zoom into mouse cursor
      const rx = event.x / viewport.width.value;
      const ry = event.y / viewport.height.value;
      viewer.value?.setView({
        w: view.value.w * (1 - bump * 2),
        h: view.value.h * (1 - bump * 2),
        x: view.value.x + view.value.w * bump * rx * 2,
        y: view.value.y + view.value.h * bump * ry * 2,
      }, {
        animationTime: 0.3,
        ease: "out",
      });
    }
  }
}

const onView = (event) => {
  if (!scene.value?.bounds.w) {
    return;
  }
  lastView.value = event;
  if (!nativeScroll.value) {
    lastNonNativeView.value = event;
  }
  if (region.value?.bounds) {
    emit("elementView", getScreenView(region.value.bounds));
  }
}

const onNav = async (event) => {
  if (event.x) {
    const valid = await navigate(event.x);
    if (!valid) {
      viewer.value?.setPendingTransition({
        t: 0.5,
        x: (lastView.value?.x - view.value?.x) / 2,
        ease: "out",
      });
      zoomOut();
    }
    return;
  }
  if (event.zoom < 0) {
    await exit();
    return;
  }
  if (event.zoom > 0) {
    emit("swipeUp");
    return;
  }
  zoomOut();
}

const onBoxSelect = async (bounds, shift) => {
  const op = shift ? "SUBTRACT" : "ADD";
  const tag = await selectBounds(op, bounds);
  emit("selectTag", tag);
}

const onLoadEnd = (event) => {
  emit('loadEnd', event);
}

const getRegionView = async (regionId) => {
  if (!scene.value || !regionId) return null;
  const region = await getRegion(scene.value.id, regionId);
  return region.bounds;
}

const getScreenView = (view) => {
  const ox = -lastView.value?.x;
  const oy = -lastView.value?.y;
  const s = viewport.width.value / lastView.value?.w;
  return {
    x: (view.x + ox) * s,
    y: (view.y + oy) * s,
    w: view.w * s,
    h: view.h * s,
  };
}

const drawViewToCanvas = (view, target) => {
  if (!target) return false;
  const canvas = viewer.value?.$el?.querySelector("canvas");
  if (!canvas) return false;
  
  const sx = view.x;
  const sy = view.y;
  const sw = view.w;
  const sh = view.h;
  const cw = target.width;
  const ch = target.height;
  const scale =
    cw/ch < sw/sh ?
      cw / sw:
      ch / sh;

  const dw = sw*scale;
  const dh = sh*scale;
  const dx = (cw - dw) * 0.5;
  const dy = (ch - dh) * 0.5;

  const c = target.getContext("2d");
  c.fillRect(0, 0, cw, ch);
  c.drawImage(canvas, sx, sy, sw, sh, dx, dy, dw, dh);
  return true;
}

// const getScrollY = () => {
//   return scrollY.value;
// }

const debugRects = computed(() => {
  return [
    {
      x: 0,
      y: 0,
      w: canvas.value.width,
      h: canvas.value.height,
      color: "#f0f0f0",
    },
    // {
    //   x: 0,
    //   y: scrollY.value,
    //   w: viewport.width.value,
    //   h: viewport.height.value,
    //   color: "green",
    // },
    {
      x: 0,
      y: scrollY.value,
      w: viewport.width.value,
      h: viewport.height.value,
      color: "blue",
    },
  ];
});

// watch(scrollY, () => {
//   console.log("nativeScrollY", nativeScrollY.value, "scrollOffset", scrollOffset.value, "scrollY", scrollY.value);
// });

// function centerNativeScroll() {
//   const half = scrollMax.value * 0.5;
//   const max = canvas.value.height - scrollMax.value - viewport.height.value;
//   console.log("centerNativeScroll", half, max);
//   let cy = scrollOffset.value + nativeScrollY.value - half;
//   if (cy < 0) {
//     cy = 0;
//   } else if (cy > max) {
//     cy = max;
//   } else {
//     scrollToPixels(half);
//   }
//   scrollOffset.value = cy;
// }

let centerScrollInterval;

defineExpose({
  getRegionView,
  // getScrollY,
  drawViewToCanvas,
  centerToBounds,
  getScreenView,
  navigate,
  exit,
})

</script>

<style>
</style>

<style scoped>

.scroll {
  position: relative;
}

.scroll .viewer {
  position: absolute;
  top: 0;
  left: 0;
}

.scroll.fullpage .viewer {
  position: absolute;
  width: 100%;
  height: 100vh;
  /* Fix for mobile browsers */
  height: calc(var(--vh, 1vh) * 100);
  margin-top: -64px;
}

.scroll.fullpage.fixed .viewer {
  position: fixed;
  margin-top: 0;
  transform: translate(0, 0) !important;
}

.scrollbar {
  position: fixed;
  right: 0;
  top: 64px;
  height: calc(100vh - 64px);
}

.controls {
  position: fixed;
  top: 0;
  left: 0;
}

.context-menu {
  position: fixed;
  width: fit-content;
}

.date-strip {
  position: fixed;
  left: 20px;
  top: 80px;
  pointer-events: none;
  opacity: 0;
  transition: opacity 2s cubic-bezier(1,0,.87,0);
}

.date-strip.visible {
  opacity: 1;
  transition: none;
}

.spinner {
  position: fixed;
  --size: 200px;
  top: calc(50% - var(--size)/2);
  left: calc(50% - var(--size)/2);
  width: var(--size);
  height: var(--size);
}

</style>
