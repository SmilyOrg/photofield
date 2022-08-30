<template>
  <div class="overlays">
    <div
      v-for="overlay in overlayPool"
      :key="overlay.poolId"
      :ref="'overlay-' + overlay.poolId"
      class="overlay"
    >
      <div
        v-if="overlay?.region?.data?.video"
        class="video-icon"
      >
      </div>
      <video-player
        v-if="overlay?.region?.data?.video"
        :region="overlay?.region"
        :full="true"
        :active="active"
        @interactive="interactive => $emit('interactive', interactive)"
      ></video-player>
    </div>
  </div>
</template>

<script>

import VideoPlayer from './VideoPlayer.vue';
import Overlay from 'ol/Overlay';

export default {

  components: {
    VideoPlayer,
  },
  
  props: {
    viewer: Object,
    overlay: Object,
    scene: Object,
    active: Boolean,
  },

  emits: ["interactive"],

  mounted() {
    this.mountViewer(this.viewer);
  },

  unmounted() {
    this.unmountViewer();
  },
  
  watch: {
    viewer(viewer) {
      this.mountViewer(viewer);
    },
    overlay: {
      immediate: true,
      handler(overlay) {
        this.updateOverlay(overlay);
      }
    },
  },

  computed: {
    overlayPool() {
      return [{
        poolId: 0,
        region: this.overlay,
      }];
    },
  },
 
  methods: {

    mountViewer(viewer) {
      if (viewer != this.mountedViewer) {
        this.unmountViewer();
        if (viewer) {
          viewer.getView().on("change:resolution", this.onResolutionChange);
        }
      }
      this.mountedViewer = viewer;
      if (!viewer) return;
      
      const ref = this.$refs["overlay-0"];
      if (ref && !this.olOverlay) {
        const overlays = viewer.getOverlays();
        const length = overlays.push(new Overlay({
          element: ref[0],
          stopEvent: false,
        }))
        const overlay = overlays.item(length - 1);
        this.olOverlay = overlay;
        this.updateOverlay(overlay);
      }
    },

    unmountViewer() {
      const viewer = this.mountedViewer;
      this.mountedViewer = null;
      if (!viewer) return;
      if (this.olOverlay) {
        viewer.removeOverlay(this.olOverlay); 
        this.olOverlay = null; 
      }
      viewer.getView().un("change:resolution", this.onResolutionChange);
    },

    onResolutionChange() {
      this.updateOverlay(this.overlay);
    },
    
    extentFromView(view) {
      if (!this.scene) throw new Error("Scene not found");
      const fullExtent = this.viewer.getView().getProjection().getExtent();
      const fw = fullExtent[2] - fullExtent[0];
      const fh = fullExtent[3] - fullExtent[1];
      const sx = fw / this.scene.bounds.w;
      const sy = fh / this.scene.bounds.h;
      const tx = view.x * sx;
      const ty = fh - view.y * sy;
      const tw = view.w * sx;
      const th = view.h * sy;
      return [tx, ty-th, tx+tw, ty];
    },

    updateOverlay(region) {
      if (!region || !region.bounds) return;
      if (!this.viewer) return;
      
      const overlay = this.olOverlay;
      if (!overlay) return;

      const extent = this.extentFromView(region.bounds);
      overlay.setPosition([extent[0], extent[3]]);
      
      const element = overlay.element;
      const resolution = this.viewer.getView().getResolution();
      element.style.width = (extent[2] - extent[0]) / resolution + "px";
      element.style.height = (extent[3] - extent[1]) / resolution + "px";
    },

  }
};
</script>

<style scoped>
.overlay {
  pointer-events: none;
}
</style>
