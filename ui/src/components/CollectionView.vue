<template>
  <div class="collection">

    <scroll-viewer
      ref="scrollViewer"
      :interactive="!stripVisible"
      :collectionId="collectionId"
      :layout="layout"
      :imageHeight="imageHeight"
      :search="search"
      :debug="debug"
      :fullpage="true"
      :scrollbar="scrollbar"
      @region="onScrollRegion"
      @scene="scrollScene = $event"
    >
    </scroll-viewer>

    <strip-viewer
      ref="stripViewer"
      class="strip"
      :class="{ visible: stripVisible }"
      :interactive="stripVisible"
      :collectionId="collectionId"
      :regionId="transitionRegionId || regionId"
      :search="search"
      :debug="debug"
      :screenView="stripView"
      :fullpage="true"
      @region="onStripRegion"
      @scene="stripScene = $event"
    >
    </strip-viewer>

  </div>
</template>

<script setup>
import { computed, nextTick, ref, toRefs, watch, watchEffect } from 'vue';
import { timeout, useTask } from 'vue-concurrency';
import { useRoute, useRouter } from 'vue-router';

import StripViewer from './StripViewer.vue';
import ScrollViewer from './ScrollViewer.vue';

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
const stripScene = ref(null);
const scenes = computed(() => {
  const scenes = [];
  if (scrollScene.value) scenes.push({
    name: "Scroll",
    ...scrollScene.value
  });
  if (stripScene.value) scenes.push({
    name: "Strip",
    ...stripScene.value
  });
  return scenes;
});
watch(scenes, scenes => emit("scenes", scenes));

const layout = computed(() => {
  return route.query.layout;
})

const imageHeight = computed(() => {
  return parseInt(route.query.image_height, 10) || (route.query.search ? 300 : 100);
})

const search = computed(() => {
  return route.query.search;
})

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

watch(regionId, (newRegionId, oldRegionId) => {
  lastRegionId.value = oldRegionId;
  const showStrip = newRegionId !== undefined;
  emit("immersive", showStrip);
  showRegion.perform(newRegionId);
}, { immediate: true });

const onStripRegion = async region => {
  if (!region) return;
  showRegion.perform(region.id);
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
