<template>
  <div class="tileViewer" ref="viewer"></div>
</template>

<script>
import OpenSeadragon from "openseadragon";
import { throttle, waitDebounce } from "../utils.js";
import { getRegions } from "../api.js";

export default {
  
  props: {
    api: String,
    scene: Object,
    interactive: Boolean,
    view: Object,
  },

  emits: ["zoom-out", "pan", "load"],

  data() {
    return {}
  },
  async created() {
    this.tempRect = new OpenSeadragon.Rect();
  },
  async mounted() {
    this.reset();
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
      this.setView(view);
    },

  },
  computed: {
    pointerTarget() {
      return this.$refs.viewer.querySelector(".openseadragon-canvas canvas");
    },
    pointerDistThreshold() {
      return this.viewer.innerTracker.clickDistThreshold;
    },
    pointerTimeThreshold() {
      return this.viewer.innerTracker.clickTimeThreshold;
    },
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
        // immediateRender: true,
        // alwaysBlend: true,
        // autoResize: false,
      });

      this.setInteractive(this.interactive);

      this.viewer.addHandler("open", () => {
        // console.log("on open view", this.view);
        this.setView(this.view);
      });

      this.viewer.addHandler("canvas-click", event => this.onCanvasClick(event));
      this.viewer.addHandler("zoom", event => this.onZoom(event));
      this.viewer.addHandler("open", () => {
        // Initializing pans a couple of times, so wait with this handler
        // until after initialization
        this.viewer.addHandler("pan", event => this.onPan(event));
      });
      this.viewer.addHandler("tile-loaded", this.onTileLoad);

    },

    async onCanvasClick(event) {
      if (!event.quick) return;
      const viewportPos = this.viewer.viewport.viewerElementToViewportCoordinates(event.position);
      const regions = await getRegions(viewportPos.x, viewportPos.y, 0, 0, this.scene.params);
      if (regions && regions.length > 0) {
        const region = regions[0];
        const scale = this.scene.width;
        this.setView({
          x: region.bounds.x * scale,
          y: region.bounds.y * scale,
          width: region.bounds.w * scale,
          height: region.bounds.h * scale,
        }, {
          animationTime: 2,
        })
      }
    },

    onZoom(event) {
      if (!this.interactive) return;
      const { zoom } = event;
      if (zoom < 0.9) {
        this.$emit("zoom-out");
      }
    },

    onPan(event) {
      const scale = this.scene.width;
      this.$emit("pan", {
        x: event.center.x * scale,
        y: event.center.y * scale,
      });
    },

    onTileLoad() {
      const loader = this.viewer.imageLoader;
      this.$emit("load", {
        inProgress: loader.jobsInProgress,
        limit: loader.jobLimit,
      });
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

    setView(view, options) {
      const scale = 1 / this.scene.width;
      const rect = this.tempRect;
      rect.x = view.x * scale;
      rect.y = view.y * scale;
      rect.width = view.width * scale;
      rect.height = view.height * scale;

      if (rect.width == 0 || rect.height == 0) return;

      function withSpeed(viewport, animationTime, callback) {
        const prevValues = {
          centerSpringX: viewport.centerSpringX.animationTime,
          centerSpringY: viewport.centerSpringY.animationTime,
          zoomSpring: viewport.zoomSpring.animationTime,
        }

        viewport.centerSpringX.animationTime =
        viewport.centerSpringY.animationTime =
        viewport.zoomSpring.animationTime =
        animationTime;

        callback();

        viewport.centerSpringX.animationTime = prevValues.centerSpringX;
        viewport.centerSpringY.animationTime = prevValues.centerSpringY;
        viewport.zoomSpring.animationTime = prevValues.zoomSpring;
      }

      if (options && options.animationTime) {
        withSpeed(this.viewer.viewport, options.animationTime, () => {
          this.viewer.viewport.fitBounds(rect, false);
        });
      } else {
        this.viewer.viewport.fitBounds(rect, options ? options.immediate : true);
      }
    },

    reset() {
      if (!this.scene.width || !this.scene.height) return;
      if (!this.viewer) {
        this.initOpenSeadragon(this.$refs.viewer);
      } else {
        var oldImage = this.viewer.world.getItemAt(0);
        const newSource = this.getTiledImage();
        this.viewer.addTiledImage({
          tileSource: newSource,
          success: () => {
            if (oldImage) this.viewer.world.removeItem(oldImage);
          }
        });
      }
    },
    
    getTiledImage() {
      const tileSize = 256;
      const minLevel = 0;
      const maxLevel = 30;
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
.tileViewer {
  width: 100%;
  height: 100%;
}
</style>
