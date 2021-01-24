<template>
  <div class="container">
    <div class="tileViewer" ref="viewer"></div>
  </div>
</template>

<script>
import OpenSeadragon from "openseadragon";
import { throttle, waitDebounce } from "../utils.js";
import { getRegions, getTileUrl } from "../api.js";

export default {
  
  props: {
    api: String,
    scene: Object,
    interactive: Boolean,
    tileSize: Number,
  },

  emits: ["zoom", "click", "view", "load", "key-down"],

  data() {
    return {
      view: {
        x: 0,
        y: 0,
        w: 0,
        h: 0,
      }
    }
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

    tileSize() {
      this.reset();
    },

    interactive(interactive) {
      if (!this.viewer) return;
      this.setInteractive(interactive);
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
        // blendTime: 0.3,
        imageLoaderLimit: 10,
        mouseNavEnabled: this.interactive,
        // preload: true,
        // autoResize: false,
        // smoothTileEdgesMinZoom: Infinity,
        // placeholderFillStyle: "#FF8800"
        // debugMode: true,
        // immediateRender: true,
        // imageSmoothingEnabled: false,
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

      this.viewer.innerTracker.keyDownHandler = null;

    },

    async onCanvasClick(event) {
      if (!this.interactive) return;
      if (!event.quick) return;
      const viewportPos = this.viewer.viewport.viewerElementToViewportCoordinates(event.position);
      const scale = this.scene.width;
      this.$emit("click", {
        x: viewportPos.x * scale,
        y: viewportPos.y * scale,
      });
    },

    onZoom(event) {
      if (!this.interactive) return;
      this.$emit("zoom", event.zoom);
      this.onPan();
    },

    onPan() {
      if (!this.interactive) return;
      const scale = this.scene.width;
      const bounds = this.viewer.viewport.getBounds();
      this.emittedView = {
        x: bounds.x * scale,
        y: bounds.y * scale,
        w: bounds.width * scale,
        h: bounds.height * scale,
      };
      this.$emit("view", this.emittedView);
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
      if (interactive) {
        element.focus();
      }
    },

    setView(view, options) {

      if (!this.viewer) {
        console.warn("Viewer not initialized yet, setting pending view", view);
        this.pendingView = { view, options };
        return;
      }

      if (this.scene.width == 0) {
        console.warn("Scene has zero width, ignoring", this.scene);
        return;
      }

      if (this.pendingView) {
        view = this.pendingView.view;
        options = this.pendingView.options;
        this.pendingView = null;
        console.warn("Using pending view", view);
      }

      this.view = view;

      const scale = 1 / this.scene.width;
      const rect = this.tempRect;
      rect.x = view.x * scale;
      rect.y = view.y * scale;
      rect.width = view.w * scale;
      rect.height = view.h * scale;

      if (rect.width == 0 || rect.height == 0) {
        console.warn("View has zero area, ignoring", rect);
        return;
      }

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
      const tileSize = this.tileSize;
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
          return getTileUrl(level, x, y, tileSize, this.scene.params);
        }
      }
    },

  }
};
</script>

<style scoped>

.container, .tileViewer {
  width: 100%;
  height: 100%;
}

</style>
