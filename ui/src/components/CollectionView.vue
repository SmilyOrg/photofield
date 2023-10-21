<template>
  <div class="collection">

    <map-viewer
      v-if="layout == 'MAP'"
      ref="mapViewer"
      :interactive="true"
      :collectionId="collectionId"
      :layout="layout"
      :sort="sort"
      :imageHeight="imageHeight"
      :search="search"
      :debug="debug"
      :selectTagId="selectTagId"
      @selectTagId="onSelectTagId"
      @region="onMapRegion"
      @scene="mapScene = $event"
      @search="onSearch"
    >
    </map-viewer>

    <scroll-viewer
      v-else
      ref="scrollViewer"
      :interactive="!stripVisible"
      :collectionId="collectionId"
      :layout="layout"
      :sort="sort"
      :imageHeight="imageHeight"
      :search="search"
      :debug="debug"
      :fullpage="true"
      :scrollbar="scrollbar"
      :selectTagId="selectTagId"
      @selectTagId="onSelectTagId"
      @region="onScrollRegion"
      @scene="scrollScene = $event"
      @search="onSearch"
    >
    </scroll-viewer>

    <strip-viewer
      ref="stripViewer"
      class="strip"
      :class="{ visible: stripVisible }"
      :interactive="stripVisible"
      :collectionId="collectionId"
      :sort="sort"
      :regionId="transitionRegionId || regionId"
      :search="search"
      :debug="debug"
      :screenView="stripView"
      :fullpage="true"
      @region="onStripRegion"
      @scene="stripScene = $event"
      @search="onSearch"
    >
    </strip-viewer>

  </div>
</template>

<script setup>
import { computed, nextTick, ref, toRefs, watch } from 'vue';
import { timeout, useTask } from 'vue-concurrency';
import { useRoute, useRouter } from 'vue-router';

import StripViewer from './StripViewer.vue';
import ScrollViewer from './ScrollViewer.vue';
import MapViewer from './MapViewer.vue';
import { useApi } from '../api';

const props = defineProps([
  "collectionId",
  "regionId",
  "fullpage",
  "scrollbar",
]);

const emit = defineEmits({
  load: null,
  tasks: null,
  immersive: immersive => typeof immersive == "boolean",
  scene: null,
  scenes: null,
  reindex: null,
});

const {
  collectionId,
  regionId,
} = toRefs(props);

const scrollViewer = ref(null);
const stripViewer = ref(null);
const stripView = ref(null);
const lastScrollRegion = ref(null);
const lastStripRegion = ref(null);

const route = useRoute();
const router = useRouter();

const initWithStrip = !!regionId.value;
const stripVisible = ref(initWithStrip);
const lastRegionId = ref(null);
const transitionRegionId = ref(null);

const scrollScene = ref(null);
const mapScene = ref(null);
const stripScene = ref(null);
const scenes = computed(() => {
  const scenes = [];
  if (scrollScene.value) scenes.push({
    name: "Scroll",
    ...scrollScene.value
  });
  if (mapScene.value) scenes.push({
    name: "Map",
    ...stripScene.value
  });
  if (stripScene.value) scenes.push({
    name: "Strip",
    ...stripScene.value
  });
  return scenes;
});
watch(scenes, scenes => emit("scenes", scenes));

const { data: collection } = useApi(
  () => collectionId.value && `/collections/${collectionId.value}`
);

const layout = computed(() => {
  return route.query.layout || collection.value?.layout || undefined;
})

const selectTagId = computed(() => {
  return route.query.select_tag || undefined;
})

const onSelectTagId = (id) => {
  router.replace({
    query: {
      ...route.query,
      select_tag: id,
    }
  });
}

const sort = computed(() => {
  switch (layout.value) {
    case "TIMELINE":
      return "-date";
    default:
      return "+date";
  }
})

const imageHeight = computed(() => {
  return parseInt(route.query.image_height, 10) || (route.query.search ? 300 : 100);
})

const search = computed(() => {
  return route.query.search;
})

const onSearch = (search) => {
  router.push({
    query: {
      ...route.query,
      search,
    }
  });
}

const debug = computed(() => {
  const v = {};
  for (const key in route.query) {
    if (key.startsWith("debug_")) {
      v[key] = route.query[key];
    }
  }
  return v;
});

const showRegion = useTask(function*(_, regionId) {
  if (regionId) {
    if (stripVisible.value) return;

    let view =
      lastScrollRegion.value?.id == regionId &&
      lastScrollRegion.value?.bounds;
      
    if (!view) {
      view = yield scrollViewer.value.getRegionView(regionId);
    }

    view = scrollViewer.value.getScreenView(view);
    yield stripViewer.value?.zoomInFromView(view);

    stripVisible.value = true;
    transitionRegionId.value = null;
    if (lastStripRegion.value?.id != regionId) {
      scrollViewer.value.drawViewToCanvas(view, stripViewer.value?.getCanvas());
    }

    yield nextTick();
    stripViewer.value.focus();

  } else {
    let view = 
      lastScrollRegion.value?.id == lastStripRegion.value.id &&
      lastScrollRegion.value?.bounds;

    if (!view) {
      view = yield scrollViewer.value?.getRegionView(lastStripRegion.value.id);
    }
    if (!view) return;

    if (lastScrollRegion.value?.id != lastStripRegion.value?.id) {
      yield scrollViewer.value?.centerToBounds(view);
    }

    view = scrollViewer.value.getScreenView(view);
    yield stripViewer.value?.zoomOutFromView(view);

    yield timeout(300);
    
    stripVisible.value = false;
    transitionRegionId.value = null;
  }
}).restartable();

function showRegionImmediate(regionId) {
  if (regionId) {
    stripVisible.value = true;
  } else {
    stripVisible.value = false;
  }
}

watch(regionId, (newRegionId, oldRegionId) => {
  lastRegionId.value = oldRegionId;
  const showStrip = newRegionId !== undefined;
  emit("immersive", showStrip);
  showRegionImmediate(newRegionId);
  // showRegion.perform(newRegionId);
}, { immediate: true });

const onStripRegion = async region => {
  if (!region) return;
  // showRegion.perform(region.id);
  showRegionImmediate(region.id);
  lastStripRegion.value = region;
}

const onScrollRegion = async (region) => {
  lastScrollRegion.value = region;
  router.push({
    name: "region",
    params: {
      collectionId: collectionId.value,
      regionId: region?.id,
    },
    query: route.query,
  });
}

const onMapRegion = async (region) => {
  const stripRegion = await stripViewer.value?.getRegionIdFromFileId(region?.data?.id);
  if (!stripRegion) {
    console.error("No strip region found for", region);
    return;
  }
  router.push({
    name: "region",
    params: {
      collectionId: collectionId.value,
      regionId: stripRegion?.id,
    },
    query: route.query,
  });
}
</script>

<style scoped>

.strip {
  visibility: hidden;
  transition: none;
  pointer-events: none;
}

.strip.visible {
  visibility: visible;
  pointer-events: all;
}

</style>
