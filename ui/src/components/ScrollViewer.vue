<template>
  <div class="scroll" :class="{ fullpage, fixed: false }">

    <tile-viewer
      class="viewer"
      ref="viewer"
      :style="{
        transform: `translate(0, ${nativeScrollY + scrollDelta * scrollDt}px)`
      }"
      :scene="scene"
      :view="view"
      :selectTag="selectTag"
      :debug="debug"
      :tileSize="512"
      :preloadView="preloadView"
      :preloadLowRes="!!region"
      :interactive="interactive"
      :pannable="!nativeScroll && interactive"
      :zoomable="!nativeScroll && interactive"
      :zoom-transition="true"
      :focus="!!region"
      :crossNav="!!region"
      :viewport="viewport"
      :qualityPreset="qualityPreset"
      @click="onClick"
      @view="onView"
      @nav="onNav"
      @wheel="onWheel"
      @contextmenu.prevent="onContextMenu"
      @load-end="onLoadEnd"
      @keydown.esc="onEscape"
      @box-select="onBoxSelect"
      @viewer="emit('viewer', $event)"
    ></tile-viewer>

    <Spinner
      class="spinner"
      :total="scene?.file_count"
      :speed="loadSpeed"
      :divider="10000"
      :loading="scene?.loading"
    ></Spinner>

    <DateStrip
      class="date-strip"
      :class="{ visible: Math.abs(scrollDelta) > viewport.height.value * 8 }"
      :date="scrollDate"
    ></DateStrip>

    <div
      class="virtual-canvas"
      :style="{ height: (nativeHeight - 64) + 'px' }">
    </div>
    
    <Scrollbar
      v-if="!region"
      class="scrollbar"
      :y="scrollY"
      :max="scrollMax"
      :timestamps="timestamps"
      @change="scrollToPixels"
    ></Scrollbar>

    <RegionMenu
      v-if="contextRegion"
      class="context-menu"
      ref="contextMenu"
      :scene="scene"
      :region="contextRegion"
      :tileSize="512"
      @close="closeContextMenu()"
      @search="emit('search', $event)"
    ></RegionMenu>
  </div>
</template>

<script setup>
import { useEventBus, watchDebounced } from '@vueuse/core';
import { computed, nextTick, onMounted, onUnmounted, ref, toRefs, watch } from 'vue';
import { getRegion, getRegions, useScene, useApi, getRegionClosestTo } from '../api';
import { useSeekableRegion, useViewport, useContextMenu, useTags, useTimestamps, useTimestampsDate, useRegion } from '../use.js';
import DateStrip from './DateStrip.vue';
import RegionMenu from './RegionMenu.vue';
import Spinner from './Spinner.vue';
import TileViewer from './TileViewer.vue';
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

const preloadRegionId = ref(null);
const lastView = ref(null);
const lastNonNativeView = ref(null);
let lastLoadedScene = null;
let lastFocusFileId = null;

const nativeHeight = computed(() => {
  if (!canvas.value?.height) {
    return 0;
  }
  return Math.min(100000, canvas.value.height);
});

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
    updateFocusFile(null);
    scrollToPixels(0);
  }
  lastLoadedScene = newScene;
  emit("scene", newScene);
});

const {
  items: focusRegions,
} = useApi(() =>
  scene.value && !scene.value.loading && focusFileId.value &&
  `/scenes/${scene.value?.id}/regions?file_id=${focusFileId.value}`
);

const focusRegion = computed(() => {
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

const { region: preloadRegion } = useRegion({ scene, id: preloadRegionId });
const preloadView = computed(() => {
  if (!preloadRegion.value) return null;
  return preloadRegion.value.bounds;
});

watch(region, async newRegion => {
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
  openEvent: contextEvent,
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

const nativeScrollY = ref(window.scrollY);
function nativeScrollTo(y) {
  window.scrollTo(0, y);
  nativeScrollY.value = y;
}

function scrollToPixels(y) {
  if (nativeHeight.value <= 0) {
    return;
  }
  y = Math.max(0, Math.min(scrollMax.value, y));
  const maxOffset = Math.max(0, scrollMax.value - nativeHeight.value + viewport.height.value);
  const nativeScrollTarget = nativeHeight.value * 0.5;
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
  const oldy = scrollY.value;
  scrollY.value = nativeScrollY.value + scrollOffset;
  updateScrollDelta(scrollY.value, oldy);
  // Scroll is treated as instantaneous and not native
  // This prevents viewer native scroll offset rendering adjustment in CSS transform
  scrollDt.value = 0;
}

function updateScrollFromNative(y) {
  const actionDistanceRatio = 0.1;
  if (nativeHeight.value <= 0) {
    return;
  }
  const maxOffset = Math.max(0, scrollMax.value - nativeHeight.value + viewport.height.value);
  const actionDist = nativeHeight.value * actionDistanceRatio;
  const nativeScrollTarget = nativeHeight.value * 0.5;
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
  const oldy = scrollY.value;
  scrollY.value = nativeScrollY.value + scrollOffset;
  updateScrollDelta(scrollY.value, oldy);
}

function onWindowScroll() {
  if (Math.abs(nativeScrollY.value - window.scrollY) < 1) {
    return;
  }
  nativeScrollY.value = window.scrollY;
}

watch(regionId, (newId, oldId) => {
  document.documentElement.classList.toggle("no-scroll", !!newId);
  if (newId !== oldId && newId !== undefined && oldId !== undefined) {
    const nid = parseInt(newId, 10);
    const oid = parseInt(oldId, 10);
    const delta = nid - oid;
    preloadRegionId.value = nid + delta;
  }
}, { immediate: true });

onMounted(() => {
  window.addEventListener("scroll", onWindowScroll);
  document.documentElement.classList.add("hide-scrollbar");
});
onUnmounted(() => {
  window.removeEventListener("scroll", onWindowScroll);
  document.documentElement.classList.remove("hide-scrollbar");
  document.documentElement.classList.remove("no-scroll");
});

const scrollDelta = ref(0);
const scrollDt = ref(0);

const scrollMax = computed(() => {
  return Math.max(0, canvas.value.height - viewport.height.value);
});

const scrollRatio = computed(() => {
  return scrollY.value / scrollMax.value;
});

let lastScrollTime = 0;
let scrollDeltaResetTimer = null;

function updateScrollDelta(y, oldy) {
  const now = Date.now();
  const dt = (now - lastScrollTime) * 1e-3;
  lastScrollTime = now;
  if (dt == 0 || dt > 0.2) {
    return;
  }
  scrollDelta.value = (y - oldy) / dt;
  scrollDt.value = dt;
  clearTimeout(scrollDeltaResetTimer);
  scrollDeltaResetTimer = setTimeout(resetScrollDelta, 100);
}

function resetScrollDelta() {
  scrollDelta.value = 0;
}

watch(nativeScrollY, y => {
  if (!nativeScroll.value) return;
  if (!canvas.value.height) return;
  if (!viewport.height.value) return;
  updateScrollFromNative(scrollOffset + y);
});

watchDebounced(scrollY, async (sy) => {
  if (!scene.value) return;
  if (!view.value || !view.value.w || !view.value.h) return;
  if (sy < 500) {
    updateFocusFile(null);
    return;
  }
  const { x, y, w, h } = view.value;
  const center = await getRegionClosestTo(
    scene.value.id,
    x, y + h * focusScreenRatioY,
  );
  const fileId = center?.data?.id;
  if (!fileId) return;
  updateFocusFile(fileId);
}, { debounce: 1000 });

function updateFocusFile(id) {
  if (id == focusFileId.value) return;
  lastFocusFileId = id;
  emit("focusFileId", id);
}

const timestamps = useTimestamps({ scene, height: viewport.height });
const scrollDate = useTimestampsDate({ timestamps, ratio: scrollRatio });

const view = computed(() => {
  if (region.value) {
    return region.value.bounds;
  }

  return {
    x: 0,
    y: scrollY.value + scrollDelta.value * scrollDt.value,
    w: viewport.width.value,
    h: viewport.height.value,
  }
});

watch([focusRegion, scrollMax], async ([focusRegion, _]) => {
  if (!focusRegion) return;
  if (canvas.value.height <= 1) return;
  if (regionId.value) return;
  if (focusRegion.data.id == lastFocusFileId) {
    lastFocusFileId = null;
    return;
  }
  const bounds = focusRegion.bounds;
  scrollToPixels(bounds.y + bounds.h * 0.5 - viewport.height.value * focusScreenRatioY);
});

const {
  selectBounds
} = useTags({
  supported: tagsSupported,
  selectTag,
  collectionId,
  scene,
});

const onClick = async (event) => {
  if (contextEvent.value) {
    closeContextMenu();
    return;
  }
  if (!event) return false;
  if (region.value) return false;
  const pos = viewer.value?.elementToViewportCoordinates(event);
  if (!pos) return false;
  if (tagsSupported.value && (selectTag.value || event.ctrlKey)) {
    const tag = await selectBounds("INVERT", {
      x: pos.x,
      y: pos.y,
      w: 0,
      h: 0,
    });
    emit("selectTag", tag);
    return false;
  }
  const regions = await getRegions(scene.value?.id, pos.x, pos.y, 0, 0);
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


defineExpose({
  getRegionView,
  drawViewToCanvas,
  centerToBounds,
  getScreenView,
  navigate,
  exit,
})

</script>

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
