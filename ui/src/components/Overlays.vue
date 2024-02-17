<template>
  <div class="overlays">
    <div
      ref="overlayRef"
      class="overlay"
    >
      <template v-if="videoOverlay">
        <div class="video-icon">
        </div>
        <video-player
          :region="overlay"
          :full="true"
          :active="active"
          @interactive="interactive => $emit('interactive', interactive)"
        ></video-player>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, onUnmounted, watch, computed, toRefs } from 'vue';
import VideoPlayer from './VideoPlayer.vue';
import Overlay from 'ol/Overlay';
import { useRegion } from '../use';

const props = defineProps({
  viewer: Object,
  regionId: String,
  scene: Object,
  active: Boolean,
});

const {
  viewer,
  regionId,
  scene,
  active
} = toRefs(props);

defineEmits(["interactive"]);

const {
  region,
} = useRegion({ scene, id: regionId });

const overlay = ref(null);
watch(region, (newRegion, oldRegion) => {
  if (newRegion?.data?.id == oldRegion?.data?.id) return;
  overlay.value = newRegion;
}, { immediate: true });

const overlayRef = ref(null);

const videoOverlay = computed(() => {
  if (!overlay.value?.data?.video) return null;
  if (!viewer.value) return null;
  if (!overlayRef.value) return null;
  return {
    ref: overlayRef.value,
    overlay: overlay.value,
  };
});
const addedOverlay = ref(null);

function addOverlay(viewer, overlay) {
  if (!overlay) return;
  if (!viewer) return;
  const overlays = viewer.getOverlays();
  const length = overlays.push(new Overlay({
    element: overlay.ref,
    stopEvent: false,
  }));
  addedOverlay.value = overlays.item(length - 1);
  updateOverlay();
}

function removeOverlay(viewer) {
  if (!viewer) return;
  if (!addedOverlay.value) return;
  const removed = viewer.removeOverlay(addedOverlay.value);
  if (!removed) {
    console.error("failed to remove overlay", addedOverlay.value);
  }
  addedOverlay.value = null;
}

onUnmounted(() => {
  removeOverlay(viewer.value);
});

watch([viewer, videoOverlay], ([newViewer, newOverlay], [oldViewer, oldOverlay]) => {
  const overlaysEqual = newOverlay?.ref == oldOverlay?.ref && newOverlay?.overlay.id == oldOverlay?.overlay.id;
  if (!overlaysEqual) {
    removeOverlay(oldViewer);
    addOverlay(newViewer, newOverlay);
  }
  if (newViewer != oldViewer) {
    if (oldViewer) {
      oldViewer.getView().un("change:resolution", onResolutionChange);
    }
    if (newViewer) {
      newViewer.getView().on("change:resolution", onResolutionChange);
    }
    removeOverlay(oldViewer);
    addOverlay(newViewer, newOverlay);
  }
}, { immediate: true });

function onResolutionChange() {
  updateOverlay();
}

function updateOverlay() {
  const region = overlay.value;
  
  if (!region?.bounds) return;
  if (!viewer.value) return;
  
  const ov = addedOverlay.value;
  if (!ov) return;

  const extent = extentFromView(viewer.value, scene.value, region.bounds);
  ov.setPosition([extent[0], extent[3]]);

  const element = ov.element;
  const resolution = viewer.value.getView().getResolution();
  element.style.width = (extent[2] - extent[0]) / resolution + "px";
  element.style.height = (extent[3] - extent[1]) / resolution + "px";
}

function extentFromView(viewer, scene, view) {
  if (!scene) throw new Error("Scene not found");
  const fullExtent = viewer.getView().getProjection().getExtent();
  const fw = fullExtent[2] - fullExtent[0];
  const fh = fullExtent[3] - fullExtent[1];
  const sx = fw / scene.bounds.w;
  const sy = fh / scene.bounds.h;
  return [
    fullExtent[0] + view.x * sx,
    fullExtent[3] - (view.y + view.h) * sy,
    fullExtent[0] + (view.x + view.w) * sx,
    fullExtent[3] - view.y * sy,
  ];
}

</script>

<style scoped>
</style>
