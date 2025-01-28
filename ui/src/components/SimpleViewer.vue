<template>
  <div class="simple">
    <tile-viewer
      class="viewer"
      ref="viewer"
      :scene="scene"
      :tileSize="512"
      :interactive="true"
      :pannable="true"
      :zoomable="true"
    ></tile-viewer>
  </div>
</template>

<script setup>
import { useEventBus, watchDebounced } from '@vueuse/core';
import { computed, nextTick, onMounted, onUnmounted, ref, toRefs, watch } from 'vue';
import { getRegion, getRegions, useScene, useApi, getRegionClosestTo } from '../api';
import { useSeekableRegion, useViewport, useContextMenu, useTags, useTimestamps, useTimestampsDate } from '../use.js';
import DateStrip from './DateStrip.vue';
import RegionMenu from './RegionMenu.vue';
import Spinner from './Spinner.vue';
import TileViewer from './TileViewer.vue';
import Scrollbar from './Scrollbar.vue';

const props = defineProps({
  collectionId: String,
  layout: String,
  sort: String,
  imageHeight: Number,
  search: String,
  selectTag: Object,
  debug: Object,
  tweaks: String,
});

const emit = defineEmits({})

const {
  regionId,
  focusFileId,
  collectionId,
  layout,
  sort,
  imageHeight,
  search,
  selectTag,
  tweaks,
} = toRefs(props);

const viewer = ref(null);
const viewport = useViewport(viewer);

const { scene } = useScene({
  layout,
  sort,
  collectionId,
  imageHeight,
  viewport,
  search,
  tweaks,
});

</script>

<style scoped>

.simple {
  position: relative;
}

.simple .viewer {
  position: absolute;
  width: 100%;
  height: 100vh;
  /* Fix for mobile browsers */
  height: calc(var(--vh, 1vh) * 100);
  margin-top: -64px;
}

</style>
