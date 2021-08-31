<template>
  <div class="overlays">
    <div
      v-for="overlay in overlayPool"
      :key="overlay.poolId"
      :id="'overlay-' + overlay.poolId"
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

import OpenSeadragon from "openseadragon";
import VideoPlayer from './VideoPlayer.vue';

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

  async created() {
    this.tempRect = new OpenSeadragon.Rect();
  },
  
  watch: {
    viewer: {
      immediate: true,
      handler(viewer) {
        if (!viewer) return;
        this.initViewer(viewer);
        this.updateOverlay(this.overlay);
      },
    },
    overlay: {
      immediate: true,
      handler(overlay) {
        this.updateOverlay(overlay);
      }
    }
  },

  computed: {
    overlayPool() {
      return [{
        poolId: 0,
        region: this.overlay,
      }];
    }
  },

  methods: {

    initViewer(viewer) {
      viewer.addOverlay(
        "overlay-0",
        new OpenSeadragon.Rect(
          0,
          0,
          0,
          0,
        ),
      );
    },

    updateOverlay(region) {
      if (!region) return;
      if (!this.viewer) return;
      const overlay = this.viewer.getOverlayById("overlay-0");
      
      const scale = 1 / this.scene.width;
      const rect = this.tempRect;
      rect.x = region.bounds.x * scale;
      rect.y = region.bounds.y * scale;
      rect.width = region.bounds.w * scale;
      rect.height = region.bounds.h * scale;

      overlay.update(rect);
    },

  }
};
</script>

<style scoped>
</style>
