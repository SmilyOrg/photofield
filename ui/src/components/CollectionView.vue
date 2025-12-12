<template>
  <div
    class="collection"
    ref="container"
    :class="{ showDetails }"
  >
    <page-title :title="pageTitle"></page-title>

    <response-loader
      class="response"
      :response="collectionResponse"
    ></response-loader>
    <center-message
      v-if="currentScene?.file_count === 0 && !currentScene?.loading"
      icon="image_not_supported"
      class="center-message"
    >
      <h1>No photos found</h1>
      <p>This may be because:</p>
      <ul>
        <li v-if="search">Search filters excluding all photos (check tag:, created:, or t: filters)</li>
        <li>The collection needs to be rescanned for new or updated photos</li>
        <li>No supported image files (.jpg, .png, .avif, etc.) in the collection directories</li>
        <li>Collection directories are empty or don't exist</li>
        <li>File system errors preventing access to photo directories</li>
        <li v-if="search">Multiple tag filters require photos to have ALL specified tags</li>
      </ul>
      <ui-button raised @click="emit('reindex')">Rescan photos</ui-button>
    </center-message>

    <map-viewer
      class="viewer"
      v-if="layout == 'MAP'"
      ref="mapViewer"
      :interactive="interactive"
      :collectionId="collection && collectionId"
      :regionId="regionId"
      :layout="layout"
      :sort="sort"
      :imageHeight="imageHeight"
      :search="search"
      :debug="debug"
      :selectTag="selectTagId && selectTag"
      @selectTag="onSelectTag"
      @region="onRegion"
      @scene="mapScene = $event"
      @viewport="emit('viewport', $event)"
      @search="onSearch"
      @viewer="mapTileViewer = $event"
      @swipeUp="onSwipeUp"
    >
    </map-viewer>

    <scroll-viewer
      class="viewer"
      v-if="layout != 'MAP'"
      ref="scrollViewer"
      :interactive="interactive"
      :collectionId="collection && collectionId"
      :regionId="regionId"
      :focusFileId="focusFileId"
      :layout="layout"
      :sort="sort"
      :imageHeight="imageHeight"
      :search="search"
      :debug="debug"
      :tweaks="tweaks"
      :fullpage="true"
      :scrollbar="scrollbar"
      :selectTag="selectTagId && selectTag"
      @selectTag="onSelectTag"
      @focusFileId="onFocusFileId"
      @region="onRegion"
      @elementView="lastView = $event"
      @scene="scrollScene = $event"
      @viewport="emit('viewport', $event)"
      @search="onSearch"
      @viewer="scrollTileViewer = $event"
      @swipeUp="onSwipeUp"
    >
    </scroll-viewer>
    
    <overlays
      class="overlays"
      :viewer="currentViewer"
      :active="!!regionId"
      :regionId="regionId"
      :scene="currentScene"
      @interactive="interactive = $event"
    ></overlays>

    <controls
      class="controls"
      v-if="!!regionId"
      :scene="currentScene"
      :regionId="regionId"
      @navigate="navigate($event)"
      @exit="exit()"
      @info="setDetails(!showDetails)"
    ></controls>

    <transition>
      <photo-details
        v-if="showDetails"
        class="details"
        :regionId="regionId"
        :scene="currentScene"
        @close="setDetails(false)"
      ></photo-details>
    </transition>

  </div>
</template>

<script setup>
import { computed, ref, toRefs, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import ResponseLoader from './ResponseLoader.vue';
import Controls from './Controls.vue';
import ScrollViewer from './ScrollViewer.vue';
import MapViewer from './MapViewer.vue';
import PageTitle from './PageTitle.vue';
import Overlays from './Overlays.vue';
import PhotoDetails from './PhotoDetails.vue';
import CenterMessage from './CenterMessage.vue';

import { useApi } from '../api';
import { refDebounced, useElementSize } from '@vueuse/core';

const props = defineProps([
  "collectionId",
  "regionId",
  "scrollbar",
]);

const emit = defineEmits({
  load: null,
  tasks: null,
  immersive: immersive => typeof immersive == "boolean",
  scene: null,
  viewport: null,
  scenes: null,
  reindex: null,
});

const {
  collectionId,
  regionId,
} = toRefs(props);
watch(regionId, (newRegionId) => {
  emit("immersive", newRegionId !== undefined);
}, { immediate: true });

const scrollViewer = ref(null);
const scrollTileViewer = ref(null);
const mapTileViewer = ref(null);
const interactive = ref(true);
const container = ref(null);

const route = useRoute();
const router = useRouter();

const showDetails = computed(() => {
  return route.hash == "#details";
});

const setDetails = (show) => {
  if (show) {
    router.push({
      query: route.query,
      hash: "#details"
    });
  } else {
    router.replace({
      query: route.query,
      hash: ""
    });
  }
}

const containerWidth = refDebounced(useElementSize(container).width, 200);

const currentViewer = computed(() => {
  if (layout.value === 'MAP') {
    return mapTileViewer.value;
  }
  return scrollTileViewer.value;
});

const currentScene = computed(() => {
  if (layout.value === 'MAP') {
    return mapScene.value;
  }
  return scrollScene.value;
});

const mapViewer = ref(null);
const lastView = ref(null);

const navigate = computed(() => {
  return (scrollViewer.value || mapViewer.value)?.navigate;
});

const exit = () => {
  (scrollViewer.value || mapViewer.value)?.exit();
  lastView.value = null;
}

const onSwipeUp = () => {
  if (containerWidth.value > 700) return;
  setDetails(true);
}

const scrollScene = ref(null);
const mapScene = ref(null);
const scenes = computed(() => {
  const scenes = [];
  if (scrollScene.value) scenes.push({
    name: "Scroll",
    ...scrollScene.value
  });
  if (mapScene.value) scenes.push({
    name: "Map",
    ...mapScene.value
  });
  return scenes;
});
watch(scenes, scenes => emit("scenes", scenes));

const collectionResponse = useApi(
  () => collectionId.value && `/collections/${collectionId.value}`
);
const { data: collection } = collectionResponse;

const layout = computed(() => {
  return route.query.layout || collection.value?.layout || undefined;
})

watch(currentScene, scene => emit("scene", scene));

// Expose current scene to global window for testing
watch(currentScene, (scene) => {
  if (typeof window !== 'undefined') {
    if (!window.__PHOTOFIELD__) {
      window.__PHOTOFIELD__ = {};
    }
    window.__PHOTOFIELD__.currentScene = scene;
  }
}, { immediate: true });

const selectTagId = computed(() => {
  return route.query.select_tag || undefined;
})

const focusFileId = computed(() => {
  return route.query.f || undefined;
})

async function onFocusFileId(id) {
  await router.replace({
    query: {
      ...route.query,
      f: id || undefined,
    },
    hash: route.hash,
  });
}

const {
  data: selectTag,
  mutate: selectTagMutate,
} = useApi(
  () => selectTagId.value && `/tags/${selectTagId.value}`
);

const pageTitle = computed(() => {
  if (!collection.value) {
    return "Photos";
  }
  const id = regionId.value;
  if (!id) {
    return `${collection.value.name} - Photos`;
  }
  return `#${id} - ${collection.value.name} - Photos`;
});

const onSelectTag = async (tag) => {
  await router.replace({
    query: {
      ...route.query,
      select_tag: tag?.id,
    },
    hash: route.hash,
  });
  await selectTagMutate(() => tag);
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
    },
    hash: route.hash,
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

const tweaks = computed(() => {
  return route.query.tweaks;
});

const onRegion = async (region) => {
  if (!region) return;
  router.push({
    name: "region",
    params: {
      collectionId: collectionId.value,
      regionId: region?.id,
    },
    query: route.query,
    hash: route.hash,
    replace: !!regionId.value
  });
}

</script>

<style scoped>

.collection {
  --details-width: 360px;
}

.center-message {
  position: fixed;
  top: 100px;
  left: 0;
  right: 0;
  background: var(--bg-color);
  z-index: 5;
  height: fit-content;
}

.controls {
  position: fixed;
  top: 0;
  left: 0;
}

.controls, .viewer {
  transition: top 0.2s;
  top: 0;
}

.showDetails .controls, .showDetails .viewer {
  max-width: calc(100vw - var(--details-width));
}

.details {
  display: block;
  position: fixed;
  top: 0;
  right: 0;
  /* right: calc(-1 * var(--details-width)); */
  width: var(--details-width);
  min-height: 100vh;
  max-width: 100%;
  overflow-y: auto;
  transition: right 0.2s, top 0.2s;
}

.details.v-enter-from, .details.v-leave-to {
  right: calc(-1 * var(--details-width));
}

@media (max-width: 700px) {

  .controls, .viewer {
    transition: top 0.2s;
    top: 0;
  }

  .showDetails .controls, .showDetails .viewer {
    max-width: 100vw;
    top: -100vh;
  }

  .details {
    left: 0;
    width: 100%;
    min-height: 100vh;
  }

  .details.v-enter-from, .details.v-leave-to {
    top: 100vh;
  }
}

</style>
