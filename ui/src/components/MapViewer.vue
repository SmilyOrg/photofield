<template>
  <div class="map">

    <tile-viewer
      class="viewer"
      ref="viewer"
      :scene="scene"
      :debug="debug"
      :tileSize="512"
      :interactive="interactive"
      :pannable="true"
      :zoomable="true"
      :geo="true"
      :zoom-transition="true"
      :viewport="viewport"
      :geoview="geoview"
      @geoview="onGeoview"
      @contextmenu.prevent="onContextMenu"
      @click="onClick"
    ></tile-viewer>

    <Spinner
      class="spinner"
      :total="
        scene?.load_count !== undefined ?
          scene?.load_count : scene?.file_count
      "
      :unit="scene?.load_unit || 'files'"
      :speed="loadSpeed"
      :divider="10000"
      :loading="scene?.loading"
    ></Spinner>

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
import { debounce } from 'throttle-debounce';
import ContextMenu from '@overcoder/vue-context-menu';
import { computed, ref, toRefs, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { getRegions, useScene } from '../api';
import { useContextMenu, useViewport } from '../use.js';
import RegionMenu from './RegionMenu.vue';
import Spinner from './Spinner.vue';
import TileViewer from './TileViewer.vue';
import { useEventBus } from '@vueuse/core';

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
})

const {
  interactive,
  collectionId,
  layout,
  sort,
  imageHeight,
  search,
  selectTagId,
  debug,
} = toRefs(props);

const viewer = ref(null);
const viewport = useViewport(viewer);

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
});

useEventBus("recreate-scene").on(scene => {
  if (scene?.name && scene?.name != "Map") return;
  recreateScene();
});

watch(scene, async (newScene, oldScene) => {
  if (newScene?.search != oldScene?.search) {
    scrollToPixels(0);
  }
  emit("scene", newScene);
});

const contextMenu = ref(null);
const {
  onContextMenu,
  flip: contextFlip,
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

const applyGeoview = (geoview) => {
  const [lon, lat, z] = geoview;
  router.replace({
    query: {
      ...router.currentRoute.value.query,
      p: `${lat.toFixed(7)},${lon.toFixed(7)},${z.toFixed(2)}z`,
    }
  });
}

const debouncedApplyGeoview = debounce(1000, applyGeoview);

const onGeoview = (geoview) => {
  debouncedApplyGeoview(geoview);
}

const onClick = async (event) => {
  if (!event) return false;
  const regions = await getRegions(scene.value?.id, event.x, event.y, 0, 0);
  if (regions && regions.length > 0) {
    const region = regions[0];
    emit("region", region);
    return true;
  }
  return false;
}

</script>

<style scoped>

.map {
  position: relative;
}

.map .viewer {
  position: absolute;
  width: 100vw;
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
  width: fit-content;
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