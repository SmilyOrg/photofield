<template>
  <div
    class="container"
    ref="container"
    :style="{ backgroundColor }"
    :class="{ interactive }"
    tabindex="1"
  >
    <div class="tileViewer" ref="map"></div>
    <photo-skeleton
      v-if="loading"
      class="skeleton"
      :offset="latestView">
    </photo-skeleton>
  </div>
</template>

<script>
import Map from 'ol/Map';
import XYZ from 'ol/source/XYZ';
import OSM from 'ol/source/OSM';
import TileLayer from 'ol/layer/Tile';
import View from 'ol/View';
import Projection from 'ol/proj/Projection';
import { defaults as defaultInteractions, DragBox, DragPan, MouseWheelZoom } from 'ol/interaction';
import {MAC} from 'ol/src/has.js';
import equal from 'fast-deep-equal';

import PhotoSkeleton from './PhotoSkeleton.vue';

import "ol/ol.css";
import { getTileUrl } from '../api';
import Kinetic from 'ol/Kinetic';

function ctrlWithMaybeShift(mapBrowserEvent) {
  const originalEvent = /** @type {KeyboardEvent|MouseEvent|TouchEvent} */ (
    mapBrowserEvent.originalEvent
  );
  return (
    !originalEvent.altKey &&
    (MAC ? originalEvent.metaKey : originalEvent.ctrlKey)
  );
};


export default {

  components: {
    PhotoSkeleton,
  },
  
  props: {
    api: String,
    scene: Object,
    interactive: Boolean,
    pan: Boolean,
    zoom: Boolean,
    zoomTransition: Boolean,
    kinetic: Boolean,
    tileSize: Number,
    view: Object,
    backgroundColor: String,
    selectTagId: String,
    debug: Object,
    loading: Boolean,
    viewport: Object,
  },

  emits: [
    "zoom",
    "click",
    "pointer-down",
    "view",
    "move-end",
    "reset",
    "load-end",
    "key-down",
    "viewer",
    "box-select"
  ],

  data() {
    return {
      viewer: null,
      maxZoom: 30,
    }
  },
  async created() {
  },
  async mounted() {
    this.latestView = null;
    this.reset();
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
      handler(newValue, oldValue) {
        if (equal(newValue, oldValue)) return;
        this.reload();
      },
    },

    interactive(interactive) {
      this.setInteractive(interactive);
    },

    pan: {
      immediate: true,
      handler(newValue) {
        this.dragPan?.setActive(newValue);
      }
    },

    zoom: {
      immediate: true,
      handler(newValue) {
        this.mouseWheelZoom?.setActive(newValue);
      }
    },

    kinetic(kinetic) {
      this.setKinetic(kinetic);
    },

    view(view) {
      this.setView(view);
    },

    selectTagId() {
      this.reload();
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
    },
    minViewportZoom() {
      if (!this.v || !this.viewport.width.value) {
        return 0;
      }
      const extent = this.extentFromView({
        x: 0,
        y: 0,
        w: this.viewport.width.value,
        h: this.viewport.height.value,
      });
      return this.v.getZoomForResolution(this.v.getResolutionForExtent(extent));
    },
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

    getTiledImageZoomAtSize(width, height) {
      let sceneAspect;
      if (width < height) {
        sceneAspect = width / height;
      } else {
        sceneAspect = height / width;
      }

      const power = width / this.tileSize;
      const zoom = Math.sqrt(power);
      return zoom;
    },

    createSource() {
      return new XYZ({
        tileUrlFunction: this.tileUrlFunction,
        crossOrigin: "Anonymous",
        // projection: this.projection,
        tileSize: [this.tileSize, this.tileSize],
        // wrapX: false,
        // zDirection: -1,
        // zDi
        // imageSmoothing: false,
        // interpolate: false,
        opaque: false,
        transition: 100,
        // transition: 0,
      });
    },

    createLayer(source) {
      return new TileLayer({
        preload: Infinity,
        source,
        // opacity: 0.9,
      });
    },

    initOpenLayers(element) {

      const extent = this.projectionExtent;

      const projection = new Projection({
        code: "tiles",
        units: "pixels",
        extent,
      });
      this.projection = projection;

      const source = this.createSource();
      this.source = source;

      const layer = this.createLayer(source);
      this.layer = layer;

      // Limit minimum size loaded to avoid
      // loading tiled images with very little content
      // let minZoom = 0;
      // const minTiledImageWidth = 10;
      // for (let i = 0; i < this.maxZoom; i++) {
      //   const zoomSize = this.getTiledImageSizeAtZoom(i);
      //   if (zoomSize.width >= minTiledImageWidth) {
      //     minZoom = i;
      //     break;
      //   }
      // }

      const dragPan = new DragPan();
      const mouseWheelZoom = new MouseWheelZoom();
      const dragBox = new DragBox({
        condition: ctrlWithMaybeShift,
      });
      dragBox.on('boxend', event => {
        this.onBoxSelect(event, dragBox.getGeometry().getExtent())
      });

      const interactions = defaultInteractions({
        dragPan: false,
        mouseWheelZoom: false,
        doubleClickZoom: false,
      }).extend([
        dragPan,
        mouseWheelZoom,
        dragBox,
      ]);
      this.interactions = interactions;
      this.dragPan = dragPan;
      this.dragPan.setActive(this.pan);

      this.mouseWheelZoom = mouseWheelZoom;
      this.mouseWheelZoom.setActive(this.zoom);

      this.map = new Map({
        target: element,
        // pixelRatio: 1,
        layers: [
          new TileLayer({
            source: new OSM(),
          }),
          layer,
        ],
        // layers: [
        //   new TileLayer({
        //     source: new OSM(),
        //   }),
        // ],
        // view: new View({
        //   center: [0, 0],
        //   zoom: 2,
        // }),
        view: new View({
          center: [0, 0],
          zoom: 2,
          // center: [extent[2]/2, extent[3]],
          // projection,
          // zoom: 0,
          // minZoom: 0,
          // maxZoom: this.maxZoom,
          enableRotation: false,
          // extent,
          // smoothExtentConstraint: false,
          // showFullExtent: true,
        }),
        controls: [],
        interactions,
        moveTolerance: 4,
      });
      this.map.on("click", event => this.onClick(event));
      this.map.on("movestart", event => this.onMoveStart(event));
      this.map.on("moveend", event => this.onMoveEnd(event));
      this.map.on("loadend", event => this.onLoadEnd(event));
      
      this.v = this.map.getView();

      this.v.setMinZoom(this.minViewportZoom);

      this.v.on('change:center', this.onCenterChange);
      this.v.on('change:resolution', this.onResolutionChange);

      if (this.latestView) {
        const latestView = this.latestView;
        this.latestView = null;
        this.setView(latestView);
        this.latestView = latestView;
      } else if (this.view) {
        this.setView(this.view);
      }

      this.setKinetic(this.kinetic);
      this.$emit("viewer", this.map);

    },

    setInteractive(interactive) {
      const element = this.$refs.container;
      if (interactive) {
        element.focus();
      }
    },

    setKinetic(kinetic) {
      if (!!this.dragPanKinetic == kinetic) {
        return;
      }
      this.dragPanKinetic = kinetic ? new Kinetic(-0.004, 0.1, 200) : undefined;
      if (this.dragPan) {
        this.interactions.remove(this.dragPan);
      }
      this.dragPan = new DragPan({
        kinetic: this.dragPanKinetic,
      });
      this.interactions.push(this.dragPan);
      this.dragPan.setActive(this.pan);
    },

    onClick(event) {
      if (!this.interactive) return;
      const coords = this.viewFromCoordinate(event.coordinate);
      if (!coords) return;
      this.$emit("click", {
        ...coords,
        originalEvent: event.originalEvent,
      });
    },

    onMoveStart(event) {
      this.moveStartEvent = event;
    },

    onMoveEnd(event) {
      const visibleExtent = this.v.calculateExtent(this.map.getSize());
      const view = this.viewFromExtent(visibleExtent);
      if (!view) return;
      this.$emit("move-end", view);
    },

    onLoadEnd() {
      this.$emit("load-end");
    },

    onCenterChange(event) {
      const visibleExtent = this.v.calculateExtent(this.map.getSize());
      const view = this.viewFromExtent(visibleExtent);
      if (!view) return;
      this.latestView = view;
      this.$emit("view", view);
    },

    onResolutionChange(event) {
      const visibleExtent = this.v.calculateExtent(this.map.getSize());
      const view = this.viewFromExtent(visibleExtent);
      if (!view) return;
      this.latestView = view;
      this.$emit("view", view);
    },

    onBoxSelect(event, extent) {
      const shift = event.mapBrowserEvent.originalEvent.shiftKey;
      const view = this.viewFromExtent(extent);
      if (!view) return;
      this.$emit("box-select", view, shift);
    },

    reset() {
      if (!this.scene?.bounds?.w || !this.scene?.bounds?.h) return;
      if (this.map) {
        this.map.dispose();
        this.dragPan = null;
        this.dragPanKinetic = null;
      }
      this.initOpenLayers(this.$refs.map);
    },

    reload() {
      const oldLayer = this.layer;
      const oldSource = this.source;
      const newSource = this.createSource();
      this.source = newSource;
      const newLayer = this.createLayer(newSource);
      this.layer = newLayer;
      const cleanup = () => {
        this.map.removeLayer(oldLayer);
        oldSource.dispose();
        oldLayer.dispose();
        this.map.un("loadend", cleanup);
      };
      this.map.on("loadend", cleanup);
      this.map.addLayer(newLayer);
    },

    tileUrlFunction([z, x, y], pixelRatio, proj) {
      if (!this.scene) return;
      const extra = {
        ...this.debug,
      }
      if (this.selectTagId) {
        extra.select_tag = this.selectTagId;
      }
      // console.log("tileUrlFunction", z, x, y, extra, pixelRatio, proj)
      return getTileUrl(
        this.scene.id,
        z, x, y,
        this.tileSize,
        this.backgroundColor,
        extra,
      );
    },

    elementToViewportCoordinates(eventOrPoint) {
      if (!this.map) {
        return null;
      }
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

    zoomFromView(view) {
      if (!view) return null;
      const vw = view.w;
      const vh = view.h;
      const sw = this.scene.bounds.w;
      const sh = this.scene.bounds.h;
      const [mw, mh] = this.map.getSize();
      const zw = mw / vw;
      const zh = mh / vh;
      return Math.min(zw, zh);
    },

    viewDistance(a, b) {
      const dx = b.x - a.x;
      const dy = b.y - a.y;
      return Math.sqrt(dx*dx + dy*dy);
    },

    viewFromExtent(extent) {
      if (!this.scene) return null;
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
      if (!this.scene) return null;
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

    setPendingAnimationTime(t) {
      this.pendingAnimationTime = t;
    },

    setView(view, options) {

      if (!this.map) {
        console.warn("Map not initialized yet, setting pending view", view);
        this.pendingView = { view, options };
        return;
      }

      if (!this.scene) {
        console.warn("Scene missing, view", view);
        return;
      }

      if (this.scene.loading) {
        console.warn("Scene loading, setting pending view", view);
        this.pendingView = { view, options };
        return;
      }

      if (this.scene.bounds.w == 0 || this.scene.bounds.h == 0) {
        console.warn("Scene has zero width or height, ignoring", this.scene);
        return;
      }

      if (!view && this.pendingView) {
        view = this.pendingView.view;
        options = this.pendingView.options;
        console.warn("Using pending view", view);
      }
      this.pendingView = null;

      if (!view) {
        console.warn("View missing");
        return;
      }

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
      
      if (this.zoomTransition && this.latestView) {
        const prevZoom = this.zoomFromView(this.latestView);
        const zoom = this.zoomFromView(view);
        const zoomDiff = Math.abs(zoom - prevZoom);
        if (zoomDiff > 1e-4 && !options) {
          const t = zoomDiff * 0.05;
          options = { animationTime: t }
        }
      }

      this.latestView = view;
      if (this.pendingAnimationTime && !options) {
        options = { animationTime: this.pendingAnimationTime }
      }
      this.pendingAnimationTime = null;

      if (this.v.getAnimating()) {
        this.v.cancelAnimations();
      }

      const targetExtent = this.extentFromView(view);
      
      const fitOpts = options ? {
        duration: options.animationTime*1000,
        easing: function(t) {
          return 1 - Math.pow(1 - t, 10)
        },
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

.interactive {
  cursor: pointer;
}

.skeleton {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
}

</style>
