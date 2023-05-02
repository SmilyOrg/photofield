<template>
  <div
    class="strip"
    ref="container"
    :class="{ fullpage: fullpage, fixed: true }"
  >
    <page-title :title="pageTitle"></page-title>

    <div
      class="backdrop"
      :class="{ visible: !transform }" 
    ></div>

    <tile-viewer
      class="viewer"
      ref="viewer"
      :class="{ zoomed: transform, reset: zoomReset }"
      :style="{ transform, clipPath, opacity }"
      :scene="scene"
      :view="view"
      :viewport="viewport"
      :pan="true"
      :zoom="true"
      :debug="debug"
      :kinetic="true"
      :tileSize="512"
      :interactive="interactive"
      background-color="#000000"
      @view="onView"
      @load-end="emit('loadEnd', $event)"
      @move-end="onMoveEnd"
      @viewer="overlayViewer = $event"
      @contextmenu.prevent="onContextMenu"
    ></tile-viewer>

    <overlays
      class="overlays"
      :viewer="overlayViewer"
      :overlay="region"
      :scene="scene"
      :active="true"
      ></overlays>

    <controls
      class="controls"
      v-if="region"
      :region="region"
      :scene="scene"
      @navigate="navigate($event)"
      @favorite="favorite($event)"
      @exit="resetZoomOrExit()"
      @add-tag="addTag($event)"
      @remove-tag="removeTag($event)"
    ></controls>

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
      ></RegionMenu>
    </ContextMenu>
  </div>
</template>

<script setup>
import ContextMenu from '@overcoder/vue-context-menu';
import { useEventBus, useMousePressed, useNow, useRefHistory } from '@vueuse/core';
import { computed, nextTick, ref, toRefs, watch } from 'vue';
import { useApi, useScene, getCenterRegion, postTagFiles } from '../api';
import { useSeekableRegion, useViewport, useViewDelta, useContextMenu } from '../use.js';
import { viewCenterSquared } from '../utils.js';
import Controls from './Controls.vue';
import Overlays from './Overlays.vue';
import PageTitle from './PageTitle.vue';
import RegionMenu from './RegionMenu.vue';
import TileViewer from './TileViewer.vue';

const props = defineProps({
  interactive: Boolean,
  collectionId: String,
  regionId: String,
  sort: String,
  search: String,
  debug: Object,
  fullpage: Boolean,
});

const emit = defineEmits({
  loadEnd: null,
  tasks: null,
  immersive: immersive => typeof immersive == "boolean",
  scene: null,
  reindex: null,
  region: null,
})

const {
  interactive,
  regionId,
  collectionId,
  sort,
  search,
  debug,
} = toRefs(props);

const screenView = ref(null);
const zoomReset = ref(false);

const viewer = ref(null);
const overlayViewer = ref(null);
const container = ref(null);

const {
  data: collection,
} = useApi(() => collectionId && `/collections/${collectionId.value}`);

const viewport = useViewport(container);

const { scene, recreate: recreateScene } = useScene({
  layout: ref("STRIP"),
  sort,
  collectionId,
  viewport,
  search,
});
watch(scene, scene => emit("scene", scene));

useEventBus("recreate-scene").on(scene => {
  if (scene?.name && scene?.name != "Strip") return;
  recreateScene();
});

const { region, navigate, exit, mutate: updateRegion } = useSeekableRegion({
  scene,
  collectionId,
  regionId,
});

const fileId = computed(() => region.value?.data?.id);

const favorite = async (tag) => {
  const tagId = tag?.id || "fav:r0";
  if (!fileId.value) {
    return;
  }
  await postTagFiles(tagId, {
    op: "INVERT",
    file_id: fileId.value,
  });
  await updateRegion();
}

const addTag = async (tagId) => {
  if (!fileId.value || !tagId) {
    return;
  }
  await postTagFiles(tagId, {
    op: "ADD",
    file_id: fileId.value,
  });
  await updateRegion();
}

const removeTag = async (tagId) => {
  if (!fileId.value || !tagId) {
    return;
  }
  await postTagFiles(tagId, {
    op: "SUBTRACT",
    file_id: fileId.value,
  });
  await updateRegion();
}

watch(region, r => emit("region", r), { immediate: true });

const contextMenu = ref(null);
const {
  onContextMenu,
  flip: contextFlip,
  close: closeContextMenu,
  region: contextRegion,
} = useContextMenu(contextMenu, viewer, scene);

const resetZoomOrExit = async () => {
  if (lastZoom.value > 1.0001) {
    centerToVisibleRegion();
    return;
  }
  exit();
}

const pageTitle = computed(() => {
  if (!collection.value) {
    return "Photos";
  }
  if (!region.value) {
    return `${collection.value.name} - Photos`;
  }
  return `#${region.value.id} - ${collection.value.name} - Photos`;
});


const centering = ref(false);
const centerPending = ref(null);

const centerToRegion = () => {
  const bounds = region.value?.bounds;
  if (!bounds) return;
  centering.value = true;
  viewer.value?.setView(bounds, {
    animationTime: 0.2,
  });
}

const transform = computed(() => {
  if (!viewport.width.value || !screenView.value) return "";
  const { x, y, w, h } = screenView.value;
  if (w === 0 || h === 0) return "";
  const vw = viewport.width.value;
  const vh = viewport.height.value;
  const scale =
    vw/vh < w/h ?
      w / viewport.width.value :
      h / viewport.height.value;

  const tx = -vw*0.5 + x + w*0.5
  const ty = -vh*0.5 + y + h*0.5
  
  return `translate(${tx}px, ${ty}px) scale(${scale})`;
});

const opacity = computed(() => {
  if (!viewport.width.value || !screenView.value) return 1;
  const { x, y, w, h } = screenView.value;
  if (w === 0 || h === 0) return 1;
  return 0;
});

const clipPath = computed(() => {
  if (!viewport.width.value || !screenView.value) return "inset(0 0)";
  const { x, y, w, h } = screenView.value;
  if (w === 0 || h === 0) return "inset(0 0)";
  const vw = viewport.width.value;
  const vh = viewport.height.value;
  if (w/h < vw/vh) {
    const px = (vw - w*vh/h) * 0.5;
    return `inset(0 ${px}px)`;
  } else {
    const px = (vh - h*vw/w) * 0.5;
    return `inset(${px}px 0)`;
  }
});

const centerToVisibleRegion = async (offset) => {
  if (!lastView.value) return;

  centering.value = true;
  
  const centerRegion = await getCenterRegion(
    scene.value.id,
    lastView.value.x,
    lastView.value.y,
    lastView.value.w,
    lastView.value.h,
  );
  if (!centerRegion) {
    centering.value = false;
    return null;
  }
  
  viewer.value?.setPendingAnimationTime(0.1);
  
  // If the center region is the current region
  if (centerRegion.id == region.value?.id) {
    if (offset) {
      navigate(offset);
    } else {
      centerToRegion();
    }
    return;
  }
  
  navigate(centerRegion, offset);
}

const view = computed(() => {
  return region.value?.bounds;
})

const lastView = ref(null);
const lastZoom = ref(1);
const lastMoveEndZoom = ref(1);

const { history: viewHistory } = useRefHistory(lastView, {
  capacity: 10,
});

const now = useNow();
const { pressed } = useMousePressed();
watch(pressed, pressed => {
  if (!pressed) {

    if (lastZoom.value > 1.0001) {
      return;
    }

    const { x: dx, zoom: dz } = viewDelta.value;
    if (Math.abs(dz) > 0.0001) return;
    if (dx < -0.5) {
      centerPending.value = -1;
    } else if (dx > 0.5) {
      centerPending.value = 1;
    } else if (Math.abs(dx) > 1e-4) {
      centerPending.value = 0;
    }
  }
})

const viewDelta = useViewDelta(viewHistory, viewport, now);

const onView = (view) => {
  if (centerPending.value !== null) {
    const dx = viewDelta.value.x;
    if (Math.abs(dx) < 10) {
      const offset = centerPending.value;
      centerPending.value = null;
      centerToVisibleRegion(offset);
    }
  }

  const zoom = viewport.width.value / view.w;
  lastZoom.value = zoom;
  lastView.value = view;
}

const onMoveEnd = (v) => {
  const zoomedOutLastMove = lastMoveEndZoom.value <= 1.1;
  const zoomedOut = lastZoom.value <= 1.1;
  const zoomedOutSqueeze = lastZoom.value <= 0.99;

  const viewDiffSq = viewCenterSquared(view.value, v);
  
  centering.value = false;
  if (zoomedOutLastMove && zoomedOutSqueeze) {
    // Zoom out from already-zoomed-out view
    exit();
  } else if (centerPending.value !== null || (zoomedOut && viewDiffSq > 1)) {
    centerPending.value = null;
    centerToVisibleRegion();
    console.log("center")
  }
  
  lastMoveEndZoom.value = lastZoom.value;
}

const zoomInFromView = async (view) => {
  zoomReset.value = true;
  screenView.value = view;
  await nextTick();
  zoomReset.value = false;
  screenView.value = null;
}

const zoomOutFromView = async (view) => {
  screenView.value = view;
}

const getCanvas = () => {
  return viewer.value?.$el?.querySelector("canvas");
}

const focus = () => {
  viewer.value?.$el?.focus();
}

defineExpose({
  getCanvas,
  zoomInFromView,
  zoomOutFromView,
  focus,
})

</script>

<style scoped>

.strip {
  position: relative;
}

.strip.fullpage {
  position: absolute;
  width: 100vw;
  height: 100vh;
  /* Fix for mobile browsers */
  height: calc(var(--vh, 1vh) * 100);
  margin-top: -64px;
}

.backdrop {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background-color: black;
  opacity: 0;
  transition: opacity 0.2s;
}

.backdrop.visible {
  opacity: 1;
}

.strip.fullpage.fixed {
  position: fixed;
  margin-top: 0;
}

.strip.fullpage.fixed .viewer {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  transition: opacity 0s, transform 0.3s cubic-bezier(0,1,.6,1), clip-path 0.3s step-end;
}

.strip.fullpage.fixed .viewer.zoomed {
  transition: opacity 0.3s cubic-bezier(1,0,.68,0), transform 0.3s cubic-bezier(0,1,.2,1);
}

.strip.fullpage.fixed .viewer.reset {
  transition: none;
  transform: translate(0 0) scale(1);
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


</style>
