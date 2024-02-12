<template>
  <div class="scroll" :class="{ fullpage, fixed: false }">

    <tile-viewer
      class="viewer"
      ref="viewer"
      :style="{ transform: `translate(0, ${scrollY}px)` }"
      :scene="scene"
      :view="view"
      :selectTagId="selectTagId"
      :debug="debug"
      :tileSize="512"
      :interactive="interactive"
      :pannable="!nativeScroll"
      :zoomable="!nativeScroll"
      :zoom-transition="regionTransition"
      :focus="!!region"
      :crossNav="!!region"
      :viewport="viewport"
      @click="onClick"
      @view="onView"
      @move-end="onMoveEnd"
      @nav="onNav"
      @wheel="onWheel"
      @load-end="onLoadEnd"
      @contextmenu.prevent="onContextMenu"
      @keydown.esc="onEscape"
      @box-select="onBoxSelect"
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
      :class="{ visible: scrollSpeed > viewport.height.value * 8 }"
      :date="scrollDate"
    ></DateStrip>

    <div
      class="scroller"
      ref="scroller"
    >
      <div
        class="virtual-canvas"
        :style="{ height: canvas.height + 'px' }">
      </div>
    </div>

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
import { useEventBus } from '@vueuse/core';
import { computed, nextTick, ref, toRefs, watch } from 'vue';
import { getRegion, getRegions, useScene, addTag, postTagFiles, useApi } from '../api';
import { useSeekableRegion, useScrollbar, useViewport, useContextMenu, useTimeline } from '../use.js';
import DateStrip from './DateStrip.vue';
import RegionMenu from './RegionMenu.vue';
import Spinner from './Spinner.vue';
import TileViewer from './TileViewer.vue';
import { tr } from 'date-fns/locale';

const props = defineProps({
  interactive: Boolean,
  collectionId: String,
  regionId: String,
  layout: String,
  sort: String,
  imageHeight: Number,
  search: String,
  selectTagId: String,
  debug: Object,
  fullpage: Boolean,
  scrollbar: Object,
});

const emit = defineEmits({
  loadEnd: null,
  tasks: null,
  immersive: immersive => typeof immersive == "boolean",
  scene: null,
  reindex: null,
  region: null,
  selectTagId: null,
  search: null,
  elementView: null,
})

const {
  interactive,
  regionId,
  collectionId,
  scrollbar,
  layout,
  sort,
  imageHeight,
  search,
  selectTagId,
  debug,
} = toRefs(props);

const viewer = ref(null);
const viewport = useViewport(viewer);
// const nativeScroll = ref(true);

const lastView = ref(null);
const lastNonNativeView = ref(null);

const { scene, recreate: recreateScene, loadSpeed } = useScene({
  layout,
  sort,
  collectionId,
  imageHeight,
  viewport,
  search,
});

watch(scene, async (newScene, oldScene) => {
  if (newScene?.search != oldScene?.search) {
    scrollToPixels(0);
  }
  emit("scene", newScene);
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
});

const exit = async () => {
  // const viewZoom = viewer.value.zoomFromView(view.value);
  // const moveZoom = viewer.value.zoomFromView(lastView.value);
  // const ratio = moveZoom / viewZoom;
  // if (ratio > 1.1) {
  //   zoomOut();
  //   return;
  // }
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
  if (!viewport.width.value || !viewport.height.value) {
    return { width: 1, height: 1 }
  }
  const aspectRatio =
    scene.value?.bounds?.h ?
    scene.value.bounds.w / scene.value.bounds.h :
    1;
  return {
    width: viewport.width.value,
    height: viewport.width.value / aspectRatio,
  }
});

const lastZoom = computed(() => {
  return scene.value?.bounds.w / lastView.value?.w;
});

const nativeScroll = computed(() => {
  // const lastZoom = lastZoom.value;
  // console.log(lastZoom.value);

  if (lastZoom.value > 1.2) {
    return false;
  }

  if (region.value) return false;
  return true;
});

watch(nativeScroll, async (newValue, oldValue) => {
  if (newValue == oldValue) {
    return;
  }
  if (newValue) {
    // console.log("nativey", lastView.value?.y, lastNonNativeView.value?.y);
    await centerToBounds(lastNonNativeView.value);
  }
});


const centerToBounds = async (bounds) => {
  const by = bounds.y + bounds.h * 0.5;
  const vy = viewport.height.value * 0.5;
  // nativeScroll.value = true;
  // if (region.value) {
  //   await exit();
  // }
  await nextTick();
  scrollToPixels(by - vy);
  await nextTick();
}

const onEscape = async () => {
  zoomOut();
  if (lastView.value) {
    const lastZoom = scene.value.bounds.w / lastView.value.w;
    if (lastZoom > 1.1) {
      return;
    }
  }
  if (selectTagId.value) {
    emit("selectTagId", null);
    return;
  }
}

const zoomOut = () => {
  viewer.value?.setView(view.value);
}

// const scrollSleep = computed(() => !nativeScroll.value);
const scrollSleep = computed(() => {
  return !nativeScroll.value || lastZoom.value > 1.0001;
});

const {
  y: scrollY,
  yPerSec: scrollSpeed,
  ratio: scrollRatio,
  scrollToPixels,
} = useScrollbar(scrollbar, scrollSleep);

const { date: scrollDate } = useTimeline({ scene, viewport, scrollRatio });

const view = computed(() => {
  // console.log("view", region.value);

  if (region.value) {
    return region.value.bounds;
  }

  const maxScrollY = Math.max(1, canvas.value.height - viewport.height.value);
  const sy = Math.min(scrollY.value, maxScrollY - 1);

  return {
    x: 0,
    y: sy,
    w: viewport.width.value,
    h: viewport.height.value,
  }
});

// watch(region, async (newRegion, oldRegion) => {
//   if (newRegion == null && oldRegion == null) {
//     // nativeScroll.value = false;
//   } else if (newRegion === null && oldRegion != null) {
//     // nativeScroll.value = true;
//     await centerToBounds(oldRegion.bounds);
//   }
// });

const selectBounds = async (op, bounds) => {
  if (!tagsSupported.value) return;
  let id = selectTagId.value;
  if (!id) {
    const tag = await addTag({
      selection: true,
      collection_id: collectionId.value,
    });
    id = tag.id;
  }
  const tag = await postTagFiles(id, {
    op,
    scene_id: scene.value.id,
    bounds
  })
  id = tag.id;
  emit("selectTagId", id);
}

const onClick = async (event) => {
  if (!event) return false;
  if (tagsSupported.value && (selectTagId.value || event.originalEvent.ctrlKey)) {
    await selectBounds("INVERT", {
      x: event.x,
      y: event.y,
      w: 0,
      h: 0,
    });
    return false;
  }
  const regions = await getRegions(scene.value?.id, event.x, event.y, 0, 0);
  if (regions && regions.length > 0) {
    const region = regions[0];
    emit("region", region);
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
  if (lastView.value) {
    const lastZoom = scene.value.bounds.w / lastView.value.w;
    const viewZoom = scene.value.bounds.w / view.value?.w;
    const zoom = scene.value.bounds.w / event.w;
    const zoomDiff = zoom - lastZoom;
    const zoomingOut = zoomDiff < -0.000001;
    // if (zoom <= 1.1 && zoomingOut) {
    //   exit();
    //   return;
    // }
    // if (viewZoom) {
    //   const ratio = zoom / viewZoom;
    //   console.log("ratio", ratio);
    //   if (ratio < 0.8 && zoomingOut) {
    //     exit();
    //     return;
    //   }
    // }
    // console.log("zoom", zoom, "lastZoom", lastZoom, "viewZoom", viewZoom, "zoomDiff", zoomDiff, "zoomingOut", zoomingOut);
    // if (zoom <= 1.1) {
    //   // Zoom out to native scroll
    //   if (!nativeScroll.value) {
    //     nativeScroll.value = true;
    //   }
    // } else if (zoom >= 1.0001) {
    //   // Zoom in via tileviewer movement (e.g. pinch gesture)
    //   if (nativeScroll.value) {
    //     nativeScroll.value = false;
    //   }
    // }
    // if (zoom <= 1.1 && zoomingOut) {
    //   // Zoom out to native scroll
    //   if (!nativeScroll.value) {
    //     nativeScroll.value = true;
    //   }
    // } else if (zoom >= 1.0001) {
    //   // Zoom in via tileviewer movement (e.g. pinch gesture)
    //   if (nativeScroll.value) {
    //     nativeScroll.value = false;
    //   }
    // }
  }
  lastView.value = event;
  if (!nativeScroll.value) {
    lastNonNativeView.value = event;
  }
  // console.log("y", nativeScroll.value, event.y)
  // console.log("view", Object.assign({}, view));
  // console.log("region", Object.assign({}, region.value?.bounds));
  // console.log("element", viewer.value?.elementFromView(view));
  // console.log("screen", getScreenView(region.value?.bounds));
  // const corners = viewer.value?.pixelCornersFromView(region.value?.bounds);
  // console.log(
  //   "x", corners?.tl[0],
  //   "y", corners?.tl[1],
  //   "w", corners?.br[0] - corners?.tl[0],
  //   "h", corners?.br[1] - corners?.tl[1],
  // );
  if (region.value?.bounds) {
    emit("elementView", getScreenView(region.value.bounds));
  }
}

const onMoveEnd = async (event) => {
  if (!scene.value?.bounds.w || !view.value || !viewer.value) {
    return;
  }
  // const viewZoom = scene.value.bounds.w / view.value.w;
  // const moveZoom = scene.value.bounds.w / event.w;
  const viewZoom = viewer.value.zoomFromView(view.value);
  const moveZoom = viewer.value.zoomFromView(event);
  const ratio = moveZoom / viewZoom;
  // console.log("ratio", ratio);
  
  // if (ratio < 0.7) {
  //   await exit();
  //   return;
  // }
  // if (ratio < 1.1) {
  //   zoomOut();
  //   return;
  // }
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
  zoomOut();
}

const onBoxSelect = async (bounds, shift) => {
  const op = shift ? "SUBTRACT" : "ADD";
  selectBounds(op, bounds);
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

const getScrollY = () => {
  return scrollY.value;
}

defineExpose({
  getRegionView,
  getScrollY,
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
  width: 100vw;
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
