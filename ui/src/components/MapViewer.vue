<template>
  <div class="map">

    <tile-viewer
      class="viewer"
      ref="viewer"
      :geo="true"
      :scene="scene"
      :view="view"
      :selectTag="selectTag"
      :debug="debug"
      :tileSize="512"
      :preloadView="preloadView"
      :interactive="interactive"
      :pannable="interactive"
      :zoomable="interactive"
      :zoom-transition="true"
      :focus="!!region"
      :crossNav="!!region"
      :viewport="viewport"
      @click="onClick"
      @view="onView"
      @nav="onNav"
      @pointerdown="onContextMenuPointerDown"
      @contextmenu.prevent="onContextMenu"
      @box-select="onBoxSelect"
      @viewer="emit('viewer', $event)"
    ></tile-viewer>

    <Spinner
      class="spinner"
      :total="
        scene?.load_count !== undefined ?
          scene?.load_count : scene?.file_count
      "
      :unit="scene?.load_unit"
      :speed="loadSpeed"
      :divider="10000"
      :loading="scene?.loading"
    ></Spinner>

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
import { debounce } from 'throttle-debounce';
import { computed, ref, toRefs, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { getRegions, useApi, useScene } from '../api';
import { useContextMenu, useRegion, useRegionZoom, useSeekableRegion, useTags, useViewport } from '../use.js';
import RegionMenu from './RegionMenu.vue';
import Spinner from './Spinner.vue';
import TileViewer from './TileViewer.vue';
import { useEventBus } from '@vueuse/core';
import Geoview from './openlayers/geoview.js';

const props = defineProps({
  interactive: Boolean,
  collectionId: String,
  regionId: String,
  layout: String,
  sort: String,
  imageHeight: Number,
  search: String,
  selectTag: Object,
  debug: Object,
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
  viewer: null,
  swipeUp: null,
})

const {
  interactive,
  collectionId,
  regionId,
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

const regionTransition = ref(false);

// Maps are always a square,
// so the layout is viewport-independent
const staticViewport = {
  width: ref(1000),
  height: ref(1000),
}

const { scene, recreate: recreateScene, loadSpeed } = useScene({
  layout,
  sort,
  collectionId,
  imageHeight,
  viewport: staticViewport,
  search,
  tweaks,
});

const {
  region,
  bounds: regionBounds,
  navigate,
  exit: regionExit,
} = useSeekableRegion({
  scene,
  collectionId,
  regionId,
})

const preloadRegionId = ref(null);
const lastView = ref(null);

const { region: preloadRegion } = useRegion({ scene, id: preloadRegionId });
const preloadView = computed(() => {
  if (!preloadRegion.value) return null;
  return preloadRegion.value.bounds;
});

useEventBus("recreate-scene").on(scene => {
  if (scene?.name && scene?.name != "Map") return;
  recreateScene();
});

watch(scene, async (newScene, oldScene) => {
  emit("scene", newScene);
});

watch(region, async (newRegion, oldRegion) => {
  regionTransition.value = !!((!newRegion && oldRegion) || (newRegion && !oldRegion));
}, { immediate: true });

watch(regionId, (newId, oldId) => {
  if (newId !== oldId && newId !== undefined && oldId !== undefined) {
    const nid = parseInt(newId, 10);
    const oid = parseInt(oldId, 10);
    const delta = nid - oid;
    preloadRegionId.value = nid + (delta > 0 ? 1 : -1);
  }
}, { immediate: true });

const { data: capabilities } = useApi(() => "/capabilities");
const tagsSupported = computed(() => capabilities.value?.tags?.supported);

const contextMenu = ref(null);
const {
  onContextMenu,
  onPointerDown: onContextMenuPointerDown,
  openEvent: contextEvent,
  close: closeContextMenu,
  region: contextRegion,
} = useContextMenu(contextMenu, viewer, scene);

const router = useRouter();
const route = useRoute();

const geoview = computed(() => {
  const p = route.query.p;
  if (!p) return;
  let [latstr, lonstr, zstr] = p.split(",", 3);
  if (!latstr || !lonstr || !zstr) return;
  if (zstr.endsWith("z")) {
    zstr = zstr.slice(0, -1);
  }
  
  const lat = parseFloat(latstr);
  const lon = parseFloat(lonstr);
  const z = parseFloat(zstr);
  if (isNaN(lat) || isNaN(lon) || isNaN(z)) return;
  const geoview = [lon, lat, z];

  return geoview;
});

const geoviewView = computed(() => {
  const view = Geoview.toView(geoview.value, scene.value?.bounds);
  return view;
});

const view = computed(() => {
  return regionBounds.value || geoviewView.value;
});

const applyGeoview = async (geoview) => {
  if (!geoview) return;
  const [lon, lat, z] = geoview;
  await router.replace({
    query: {
      ...router.currentRoute.value.query,
      p: `${lat.toFixed(7)},${lon.toFixed(7)},${z.toFixed(2)}z`,
    },
    hash: route.hash,
  });
}

const applyView = (view) => {
  const pg = geoview.value;
  const g = Geoview.fromView(view, scene.value?.bounds);
  
  if (Geoview.equal(g, pg)) return;
  applyGeoview(g);
}

const debouncedApplyView = debounce(1000, applyView);

const onView = (view) => {
  debouncedApplyView(view);
  lastView.value = view;
}

const regionZoom = useRegionZoom({ view: lastView, region });

const exit = async () => {
  if (selectTag.value) {
    emit("selectTag", null);
    return;
  }
  if (!region.value) {
    return;
  }
  const zoomedIn = regionZoom.value > 1.1;
  if (zoomedIn) {
    zoomOut();
    return;
  }
  const zoomedOut = regionZoom.value < 0.9;
  const g = Geoview.fromView(lastView.value, scene.value?.bounds);
  await applyGeoview([
    g[0],
    g[1],
    zoomedOut ? Math.max(1, g[2] - 1/(1+regionZoom.value)) : Math.max(1, g[2] - 3),
  ])
  await regionExit();
}

const zoomOut = () => {
  viewer.value?.setView(view.value);
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
    const region = regions[0];
    emit("region", region);
    return true;
  }
  return false;
}

const onBoxSelect = async (bounds, shift) => {
  const op = shift ? "SUBTRACT" : "ADD";
  const tag = await selectBounds(op, bounds);
  emit("selectTag", tag);
}

defineExpose({
  navigate,
  exit,
})

</script>

<style scoped>

.map {
  position: relative;
}

.map .viewer {
  position: absolute;
  width: 100%;
  height: 100vh;
  /* Fix for mobile browsers */
  height: calc(var(--vh, 1vh) * 100);
  margin-top: -64px;
}

.controls {
  position: fixed;
  top: 0;
  left: 0;
}

.context-menu {
  position: fixed;
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
