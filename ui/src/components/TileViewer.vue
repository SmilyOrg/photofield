<template>
  <div>
    <div class="viewer" ref="viewer"></div>
  </div>
</template>

<script>
import OpenSeadragon from "openseadragon";
import { throttle, waitDebounce } from "../utils.js";

export default {
  
  props: [
    "api",
    "scene",
    "interactive",
    "view",
  ],

  data() {
    return {}
  },
  async created() {
  },
  async mounted() {
    this.tempRect = new OpenSeadragon.Rect();
  },
  unmounted() {
  },
  watch: {

    scene() {
      this.reset();
    },

    interactive(interactive) {
      if (!this.viewer) return;
      this.setInteractive(interactive);
    },

    view(view) {
      if (!this.viewer) return;
      const scale = 1 / this.scene.width;
      this.tempRect.x = view.x * scale;
      this.tempRect.y = view.y * scale;
      this.tempRect.width = view.width * scale;
      this.tempRect.height = view.height * scale;
      // console.log(this.tempRect);
      this.viewer.viewport.fitBounds(this.tempRect, true);
    }

  },
  computed: {
  },
  methods: {
    initOpenSeadragon(element) {
      
      this.viewer = OpenSeadragon({
        element,
        prefixUrl: "./openseadragon/images/",
        tileSources: this.getTiledImage(),
        showNavigationControl: false,
        defaultZoomLevel: 1,
        // constrainDuringPan: true,
        viewportMargins: {
          left: 0,
          right: 0,
        },
        springStiffness: 10,
        gestureSettingsMouse: {
          clickToZoom: false,
          flickMomentum: 0.2,
        },
        gestureSettingsTouch: {
          clickToZoom: false,
          flickMomentum: 0.2,
          flickEnabled: false,
        },
        animationTime: 0.1,
        zoomPerSecond: 1.0,
        zoomPerScroll: 1.5,
        blendTime: 0.3,
        imageLoaderLimit: 10,
        mouseNavEnabled: this.interactive,
        // debugMode: true,
      });

      this.setInteractive(this.interactive);

    },

    setInteractive(interactive) {
      this.viewer.setMouseNavEnabled(interactive);
      const touchAction = interactive ? 'none' : 'auto';
      const element = OpenSeadragon.getElement(this.viewer.canvas);
      if (typeof element.style.touchAction !== 'undefined') {
        element.style.touchAction = touchAction;
      } else if (typeof element.style.msTouchAction !== 'undefined') {
        element.style.msTouchAction = touchAction;
      }
    },

    reset() {
      if (!this.scene.width || !this.scene.height) return;
      if (!this.viewer) {
        this.initOpenSeadragon(this.$refs.viewer);
      }
      var oldImage = this.viewer.world.getItemAt(0);
      const newSource = this.getTiledImage();
      this.viewer.addTiledImage({
        tileSource: newSource,
        success: () => {
          if (oldImage) this.viewer.world.removeItem(oldImage);
        }
      });
    },
    
    getTiledImage() {
      const tileSize = 256;
      const minLevel = 0;
      const maxLevel = 20;
      const power = 1 << maxLevel;
      let width = power*tileSize;
      let height = power*tileSize;
      const sceneAspect = this.scene.width / this.scene.height;
      if (sceneAspect < 1) {
        width = height * sceneAspect;
      } else {
        height = width / sceneAspect;
      }
      if (width < 1) width = 1;
      if (height < 1) height = 1;
      return {
        width,
        height,
        tileSize,
        minLevel,
        maxLevel,
        getTileUrl: (level, x, y) => {
          let url = this.api + "/tiles";
          url += "?" + this.scene.params;
          url += "&tileSize=" + tileSize;
          url += "&zoom=" + level;
          url += "&x=" + x;
          url += "&y=" + y;
          // for (const [key, value] of Object.entries(this.debug)) {
          //   url += "&debug" + key.slice(0, 1).toUpperCase() + key.slice(1) + "=" + (value ? "true" : "false");
          // }
          return url;
        }
      }
    },

  }
};
</script>

<style scoped>
.viewer {
  width: 100%;
  height: 100%;
}
</style>
