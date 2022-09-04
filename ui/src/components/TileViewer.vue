<template>
  <div class="container" ref="container" tabindex="1">
    <div class="tileViewer" ref="map"></div>
  </div>
</template>

<script>
import Map from 'ol/Map';
import XYZ from 'ol/source/XYZ';
import TileLayer from 'ol/layer/Tile';
import View from 'ol/View';
import Projection from 'ol/proj/Projection';

import "ol/ol.css";
import { getTileUrl } from '../api';

export default {
  
  props: {
    api: String,
    scene: Object,
    interactive: Boolean,
    tileSize: Number,
    view: Object,
    debug: Object,
  },

  emits: ["zoom", "click", "view", "reset", "load", "key-down", "viewer"],

  data() {
    return {
      viewer: null,
      maxZoom: 30,
      latestView: {
        x: 0,
        y: 0,
        w: 0,
        h: 0,
      }
    }
  },
  async created() {
  },
  async mounted() {

    this.reset();
  },
  unmounted() {
  },
  watch: {

    scene(newScene, oldScene) {
      if (
        newScene?.id == oldScene?.id &&
        newScene.bounds.w == oldScene.bounds.w &&
        newScene.bounds.h == oldScene.bounds.h
      ) {
        return;
      }
      this.reset();
    },

    tileSize() {
      this.reset();
    },

    debug: {
      deep: true,
      handler() {
        this.reload();
      },
    },

    interactive(interactive) {
      this.setInteractive(interactive);
    },

    view(view) {
      this.setView(view);
    },

  },
  computed: {
    pointerTarget() {
      return this.$refs.map.querySelector(".ol-layer > canvas");
    },
    pointerDistThreshold() {
      return 5;
    },
    pointerTimeThreshold() {
      return 300;
    },
    projectionExtent() {
      let { width, height } = this.getTiledImageSizeAtZoom(this.maxZoom);
      if (width < 1) width = 1;
      if (height < 1) height = 1;
      return [0, 0, width, height];
    }
  },
  methods: {

    getTiledImageSizeAtZoom(zoom) {
      const tileSize = this.tileSize;
      const power = 1 << zoom;
      let width = power*tileSize;
      let height = power*tileSize;
      const sceneAspect = this.scene.bounds.w / this.scene.bounds.h;
      if (sceneAspect < 1) {
        width = height * sceneAspect;
      } else {
        height = width / sceneAspect;
      }
      return { width, height }
    },

    initOpenLayers(element) {

      const extent = this.projectionExtent;

      const projection = new Projection({
        code: "tiles",
        units: "pixels",
        extent,
      });
      this.projection = projection;

      const source = new XYZ({
        tileUrlFunction: this.tileUrlFunction,
        crossOrigin: "Anonymous",
        projection,
        tileSize: [this.tileSize, this.tileSize],
        wrapX: false,
        // zDirection: -1,
        // zDi
        // imageSmoothing: false,
        // interpolate: false,
        opaque: true,
        transition: 100,
        // transition: 0,
      });
      this.source = source;

      const layer = new TileLayer({
        preload: Infinity,
        source,
      });
      this.layer = layer;

      // Limit minimum size loaded to avoid
      // loading tiled images with very little content
      let minZoom = 0;
      const minTiledImageWidth = 10;
      for (let i = 0; i < this.maxZoom; i++) {
        const zoomSize = this.getTiledImageSizeAtZoom(i);
        if (zoomSize.width >= minTiledImageWidth) {
          minZoom = i;
          break;
        }
      }

      const sceneSmallerThanViewport =
        this.scene.bounds.w < element.clientWidth ||
        this.scene.bounds.h < element.clientHeight;

      this.map = new Map({
        target: element,
        // pixelRatio: 1,
        layers: [layer],
        view: new View({
          center: [extent[2]/2, extent[3]],
          projection,
          zoom: 0,
          minZoom,
          maxZoom: this.maxZoom,
          enableRotation: false,
          extent,
          smoothExtentConstraint: false,
          showFullExtent: sceneSmallerThanViewport,
        }),
        controls: [],
      });
      this.map.on("click", event => this.onClick(event));
      this.map.on("movestart", event => this.onMoveStart(event));
      this.map.on("moveend", event => this.onMoveEnd(event));

      this.v = this.map.getView();

      this.setInteractive(this.interactive);
      this.$emit("viewer", this.map);

    },

    setInteractive(interactive) {
      const element = this.$refs.container;
      if (interactive) {
        element.focus();
      }
    },

    onClick(event) {
      if (!this.interactive) return;
      const coords = this.viewFromCoordinate(event.coordinate);
      this.$emit("click", coords);
    },

    onMoveStart(event) {
      this.moveStartEvent = event;
    },

    onMoveEnd(event) {
      if (!this.interactive) return;

      if (!this.scene) return;
      if (!this.moveStartEvent) throw new Error("Missing moveStartEvent");

      const startState = this.moveStartEvent.frameState.viewState;
      const endState = event.frameState.viewState;

      const zoomChange = startState.zoom != endState.zoom;
      const panChange = startState.center[0] != endState.center[0] || startState.center[1] != endState.center[1];

      // console.log(endState.zoom)

      if (!zoomChange && !panChange) {
        return;
      }

      const visibleExtent = this.v.calculateExtent(this.map.getSize());
      const view = this.viewFromExtent(visibleExtent);
      this.latestView = view;
    
      if (zoomChange) {
        const viewWidthZoom = this.scene.bounds.w / view.w;
        const viewHeightZoom = this.scene.bounds.w / view.h;
        const viewMinZoom = Math.min(viewWidthZoom, viewHeightZoom);
        this.$emit("zoom", viewMinZoom);
      }

      if (panChange) {
        this.$emit("view", view);
      }
    },

    reset() {
      if (!this.scene?.bounds?.w || !this.scene?.bounds?.h) return;
      if (this.map) {
        this.map.dispose();
      }
      this.initOpenLayers(this.$refs.map);
    },

    reload() {
      this.source.refresh();
    },

    tileUrlFunction([z, x, y], pixelRatio, proj) {
      if (!this.scene) return;
      return getTileUrl(this.scene.id, z, x, y, this.tileSize, this.debug);
    },

    elementToViewportCoordinates(eventOrPoint) {
      const coord = this.map.getEventCoordinate(eventOrPoint);
      return this.viewFromCoordinate(coord);
    },

    extentFromView(view) {
      if (!this.scene) throw new Error("Scene not found");
      const fullExtent = this.projection.getExtent();
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

    viewFromExtent(extent) {
      if (!this.scene) throw new Error("Scene not found");
      const fullExtent = this.projection.getExtent();
      const fw = fullExtent[2] - fullExtent[0];
      const fh = fullExtent[3] - fullExtent[1];
      const sx = this.scene.bounds.w / fw;
      const sy = this.scene.bounds.h / fh;
      const tx = extent[0];
      const ty = extent[3];
      const tw = extent[2]-tx;
      const th = ty-extent[1];
      return {
        x: tx * sx,
        y: (fh-ty)*sy,
        w: tw * sx,
        h: th * sy,
      }
    },

    viewFromCoordinate(coord) {
      if (!this.scene) throw new Error("Scene not found");
      const fullExtent = this.projection.getExtent();
      const fw = fullExtent[2] - fullExtent[0];
      const fh = fullExtent[3] - fullExtent[1];
      const sx = this.scene.bounds.w / fw;
      const sy = this.scene.bounds.h / fh;
      const tx = coord[0];
      const ty = coord[1];
      return {
        x: tx * sx,
        y: (fh-ty)*sy,
      }
    },

    setView(view, options) {
    
      if (!this.map) {
        console.warn("Map not initialized yet, setting pending view", view);
        this.pendingView = { view, options };
        return;
      }

      if (!this.scene) {
        console.warn("Scene missing", view);
        return;
      }

      if (this.scene.bounds.w == 0) {
        console.warn("Scene has zero width, ignoring", this.scene);
        return;
      }

      if (this.pendingView && !view) {
        view = this.pendingView.view;
        options = this.pendingView.options;
        console.warn("Using pending view", view);
      }
      this.pendingView = null;

      if (
        this.latestView && view &&
        this.latestView.x == view.x &&
        this.latestView.y == view.y &&
        this.latestView.w == view.w &&
        this.latestView.h == view.h
      ) {
        // View is already up to date, nothing to do.
        return;
      }
      
      this.latestView = view;

      const targetExtent = this.extentFromView(view);
      
      const fitOpts = options ? {
        duration: options.animationTime*1000,
      } : undefined;

      this.v.fit(targetExtent, fitOpts);
    },

  }
};
</script>

<style scoped>

.container, .tileViewer {
  width: 100%;
  height: 100%;
}

.container {
  position: relative;
  /* padding-top: 60px; */
}

</style>
