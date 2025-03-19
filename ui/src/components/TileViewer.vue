<template>
  <div
    class="container"
    ref="container"
    :style="{ backgroundColor: containerBackgroundColor }"
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
import { easeIn, easeOut, linear } from 'ol/easing';
import { defaults as defaultInteractions, DragBox, DragPan, MouseWheelZoom } from 'ol/interaction';
import { defaults as defaultControls } from 'ol/control';
import { MAC } from 'ol/has';
import Kinetic from 'ol/Kinetic';
import { get as getProjection } from 'ol/proj';
import { getBottomLeft, getTopLeft, getTopRight, getBottomRight } from 'ol/extent';

import equal from 'fast-deep-equal';
import CrossDragPan from './openlayers/CrossDragPan';
import Geoview from './openlayers/geoview.js';

import PhotoSkeleton from './PhotoSkeleton.vue';

import "ol/ol.css";
import { getTileUrl } from '../api';
import { useColorMode } from '@vueuse/core';


function ctrlWithMaybeShift(mapBrowserEvent) {
  const originalEvent = /** @type {KeyboardEvent|MouseEvent|TouchEvent} */ (
    mapBrowserEvent.originalEvent
  );
  return (
    !originalEvent.altKey &&
    (MAC ? originalEvent.metaKey : originalEvent.ctrlKey)
  );
};

function zoomEase(x) {
  return x < 0.5 ? 4 * x * x * x : 1 - Math.pow(-2 * x + 2, 3) / 2
};

export default {

  components: {
    PhotoSkeleton,
  },
  
  props: {
    api: String,
    scene: Object,
    interactive: Boolean,
    pannable: Boolean,
    zoomable: Boolean,
    zoomTransition: Boolean,
    kinetic: Boolean,
    tileSize: Number,
    view: Object,
    clipview: Object,
    crossNav: Boolean,
    focus: Boolean,
    selectTag: Object,
    debug: Object,
    loading: Boolean,
    geo: Boolean,
    viewport: Object,
    qualityPreset: String,
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
    "box-select",
    "nav",
  ],

  data() {
    return {
      viewer: null,
      maxZoom: 30,
      focusZoom: 1,
    }
  },
  async mounted() {
    this.latestView = null;
    this.lastAnimationTime = 0;
    this.reset();
  },
  setup() {
    const colorMode = useColorMode();
    return {
      colorMode,
    }
  },
  watch: {

    scene(newScene, oldScene) {
      if (
        newScene?.id != oldScene?.id ||
        newScene.bounds.w != oldScene.bounds.w ||
        newScene.bounds.h != oldScene.bounds.h
      ) {
        if (newScene?.loading) return;
        this.reset();
        return;
      }
      if (oldScene?.loading && !newScene?.loading) {
        this.reload();
        return;
      }
    },

    colorMode() {
      this.reloadMain();
    },

    geo(newGeo, oldGeo) {
      if (newGeo != oldGeo) {
        this.reset();
      }
    },

    viewport: {
      handler(viewport) {
        if (!viewport) return;
        if (!this.geo) return;
        this.reset();
      },
      deep: true,
    },

    tileSize() {
      this.reset();
    },

    debug: {
      deep: true,
      handler(newValue, oldValue) {
        if (equal(newValue, oldValue)) return;
        this.reloadMain();
      },
    },

    interactive(interactive) {
      this.setInteractive(interactive);
    },

    zoomable: {
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

    geoLayersActive(active) {
      this.map.getLayers().forEach(layer => {
        if (!layer.get("geo")) return;
        layer.setVisible(active);
      });
    },

    crossPanActive: {
      immediate: true,
      handler(active) {
        this.crossPan?.setActive(active);
      }
    },

    dragPanActive: {
      immediate: true,
      handler(active) {
        this.dragPan?.setActive(active);
      }
    },

    selectTag() {
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
      return [0, 0, width, height];
    },
    viewExtent() {
      let { width, height } = this.getTiledImageSizeAtZoom(this.maxZoom);
      return [-width*0.95, -height, width*1.95, height];
    },
    crossPanActive() {
      return this.pannable && this.crossNav && this.focusZoom < 1.1;
    },
    dragPanActive() {
      return this.pannable && !this.crossPanActive;
    },
    geoLayersActive() {
      if (!this.geo) return false;
      if (!this.focus) return true;

      return this.focusZoom < 0.99;
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
    containerBackgroundColor() {
      return this.focus && this.focusZoom > 0.99 ? "black" : null;
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

    createMainLayer() {
      const main = new TileLayer({
        properties: {
          main: true,
        },
        preload: this.geo ? 2 : Infinity,
        source: new XYZ({
          tileUrlFunction: this.tileUrlFunction,
          crossOrigin: "Anonymous",
          projection: this.projection,
          tileSize: [this.tileSize, this.tileSize],
          opaque: false,
          transition: 100,
        }),
      });
      
      if (this.geo) {
        main.on("prerender", event => {
          const ctx = event.context;
          // Fill in the transparent holes with the photos
          ctx.globalCompositeOperation = "destination-over";
        });
        
        main.on("postrender", event => {
          const ctx = event.context;
          // Restore the default
          ctx.globalCompositeOperation = "source-over";
        });
      }

      main.on("postrender", event => {
        if (!this.focus) return;

        const ctx = event.context;
        const view = this.view;
        if (!view) return;

        const size = this.map.getSize();
        const corners = this.pixelCornersFromView(view);
        const pixelRatio = window.devicePixelRatio;
        const mapw = size[0] * pixelRatio;
        const maph = size[1] * pixelRatio;
        corners.tl[0] *= pixelRatio;
        corners.tl[1] *= pixelRatio;
        corners.tr[0] *= pixelRatio;
        corners.tr[1] *= pixelRatio;
        corners.br[0] *= pixelRatio;
        corners.br[1] *= pixelRatio;
        corners.bl[0] *= pixelRatio;
        corners.bl[1] *= pixelRatio;

        const viewExtent = this.extentFromView(view);
        const viewRes = this.v.getResolutionForExtent(viewExtent);
        const refRes = viewRes * 3;
        const res = this.v.getResolution();
        const resFrac = 1 - Math.min(1, (res - viewRes) / (refRes - viewRes));
        
        const alpha = resFrac;

        const e = 1;
        
        ctx.fillStyle = `rgba(0, 0, 0, ${alpha})`;
        ctx.strokeStyle = "green";
        ctx.lineWidth = 20;
        ctx.beginPath();
        ctx.rect(0, 0, mapw, maph);
        ctx.moveTo(corners.tl[0] + e, corners.tl[1] + e);
        ctx.lineTo(corners.tr[0] - e, corners.tr[1] + e);
        ctx.lineTo(corners.br[0] - e, corners.br[1] - e);
        ctx.lineTo(corners.bl[0] + e, corners.bl[1] - e);
        ctx.closePath();
        ctx.fill("evenodd");

        if (alpha === 1 || alpha === 0) {
          return;
        }

        this.map.render();
      });
      
      return main;
    },

    createLayers() {

      const main = this.createMainLayer();

      if (this.geo) {

        const mask = new TileLayer({
          properties: {
            geo: true,
          },
          preload: 2,
          source: new XYZ({
            tileUrlFunction: this.maskUrlFunction,
            crossOrigin: "Anonymous",
            projection: this.projection,
            tileSize: [this.tileSize, this.tileSize],
            opaque: false,
            transition: 0,
          }),
        });

        const osmLayer = new TileLayer({
          properties: {
            geo: true,
          },
          preload: 2,
          source: new OSM({
            attributions: [
              'Background from <a href="https://www.openstreetmap.org/">OpenStreetMap</a>',
            ],
          }),
        });

        mask.on("prerender", event => {
          const ctx = event.context;
          // Cut out transparent holes out of the rendered map
          // using the mask
          ctx.globalCompositeOperation = "destination-out";
        });

        mask.on("postrender", event => {
          const ctx = event.context;
          // Restore the default
          ctx.globalCompositeOperation = "source-over";
        });

        return [
          osmLayer,
          mask,
          main,
        ]
      } else {
        return [
          main,
        ];
      }
    },

    initOpenLayers(element) {
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
      const crossPan = new CrossDragPan({
        centerZoom: this.geo,
      });
      crossPan.on("nav", event => {
        this.onNav(event);
      });
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
        crossPan,
        mouseWheelZoom,
        dragBox,
      ]);
      this.interactions = interactions;
      this.dragPan = dragPan;
      this.crossPan = crossPan;
      this.dragPan.setActive(this.dragPanActive);
      this.crossPan.setActive(this.crossPanActive);

      this.mouseWheelZoom = mouseWheelZoom;
      this.mouseWheelZoom.setActive(this.zoomable);

      if (this.geo) {
        this.projection = getProjection("EPSG:3857");
        this.v = new View({
          projection: this.projection,
          center: [0, 0],
          zoom: 0,
          enableRotation: false,
        });
      } else {
        this.projection = new Projection({
          code: "tiles",
          units: "pixels",
          extent: this.projectionExtent,
        });
        const extent = this.viewExtent;
        this.v = new View({
          center: [extent[2]/4, extent[3]],
          projection: this.projection,
          zoom: 0,
          minZoom: 0,
          maxZoom: this.maxZoom,
          enableRotation: false,
          extent,
          smoothExtentConstraint: false,
          showFullExtent: true,
        });
      }

      this.map = new Map({
        target: element,
        layers: this.createLayers(),
        view: this.v,
        controls: defaultControls({
          attribution: true,
          rotate: false,
          zoom: false,
        }),
        interactions,
        moveTolerance: 4,
      });

      this.map.on("click", event => this.onClick(event));
      this.map.on("movestart", event => this.onMoveStart(event));
      this.map.on("moveend", event => this.onMoveEnd(event));
      this.map.on("loadend", event => this.onLoadEnd(event));

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
      this.initZoom = this.focusZoom;

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
      this.dragPan.setActive(this.dragPanActive);
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

    onMoveEnd() {
      const visibleExtent = this.v.calculateExtent(this.map.getSize());
      const view = this.viewFromExtent(visibleExtent);
      if (!view) return;
      if (this.crossNav) {
        this.navOnZoom();
        return;
      }
      this.$emit("move-end", view);
    },

    onLoadEnd() {
      this.$emit("load-end");
    },

    onCenterChange() {
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
      if (this.focus) {
        const focuszoom = this.zoomFromView(this.view);
        const viewzoom = this.zoomFromView(view);
        const ratio = viewzoom / focuszoom;
        this.focusZoom = ratio;
      } else {
        this.focusZoom = 0;
      }
    },

    onBoxSelect(event, extent) {
      const shift = event.mapBrowserEvent.originalEvent.shiftKey;
      const view = this.viewFromExtent(extent);
      if (!view) return;
      this.$emit("box-select", view, shift);
    },

    onNav(event) {
      if (event.x) {
        const dx = event.x > 0 ? 1 : -1;
        const t = 150 / Math.abs(event.x);
        const cx = this.view.x;
        const cw = this.view.w;
        const vx = this.latestView.x;
        const vw = this.latestView.w;
        const hideFrac = (2*(cx - vx) + cw - vw) / (cw + vw);

        const show = () => {
          this.setPendingTransition({
            t,
            x: -dx * this.latestView.w,
            ease: "linear",
          });
          this.$emit("nav", {
            x: dx,
            y: 0,
          });
        };
        const hideT = t * (1 - Math.abs(hideFrac));
        if (Math.abs(hideFrac) < 1) {
          const hideX = cx + (cw + vw) * 0.5 * dx;
          this.setView({
            x: hideX,
            y: this.view.y,
            w: this.view.w,
            h: this.view.h,
          }, {
            animationTime: hideT,
            ease: "linear",
          });
          clearTimeout(this.navTimer);
          this.navTimer = setTimeout(show, hideT * 1000);
        } else {
          show();
        }
        return;
      }
      if (event.y < 0) {
        this.$emit("nav", {
          zoom: -1,
        });
        return;
      }
      if (event.y > 0) {
        this.$emit("nav", {
          zoom: 1,
        });
        return;
      }
      if (event.interrupted) {
        this.navOnZoom(event);
        return;
      }
      this.setView(this.view, {
        animationTime: 0.3,
        ease: "out",
      });
    },

    navOnZoom() {
      const ratio = this.focusZoom;
      if (Math.abs(1 - ratio) < 1e-4) {
        return;
      }
      if (Math.abs(this.initZoom - this.focusZoom) < 1e-4) {
        return;
      }
      if (ratio < 0.8) {
        this.$emit("nav", { zoom: -1 });
      } else if (ratio < 1.2) {
        this.$emit("nav", { x: 0, y: 0 });
      }
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
      if (!this.map) return;
      const oldLayers = this.map.getLayers().getArray().slice();
      const newLayers = this.createLayers();
      const cleanup = () => {
        for (const old of oldLayers) {
          this.map.removeLayer(old);
          old?.getSource()?.dispose();
          old?.dispose();
        }
        this.map.un("loadend", cleanup);
      };
      this.map.on("loadend", cleanup);
      for (const newLayer of newLayers) {
        this.map.addLayer(newLayer);
      }
    },

    reloadMain() {
      const oldLayers = this.map.getLayers();
      const oldMain = oldLayers.getArray().find(l => l.get("main"));
      const newMain = this.createMainLayer();
      const cleanup = () => {
        if (oldMain) {
          this.map.removeLayer(oldMain);
          oldMain?.getSource()?.dispose();
          oldMain?.dispose();
        }
        this.map.un("loadend", cleanup);
      };
      this.map.on("loadend", cleanup);
      this.map.addLayer(newMain);
    },

    tileUrlFunction([z, x, y]) {
      if (!this.scene) return;
      const extra = {
        ...this.debug,
      }
      if (this.selectTag) {
        extra.select_tag = this.selectTag.id;
        extra.select_tag_etag = this.selectTag.etag;
      }
      if (this.qualityPreset) {
        extra.quality_preset = this.qualityPreset;
      }
      if (this.colorMode === "dark") {
        extra.color = "#FFFFFF";
        extra.background_color = "#222222";
      }
      return getTileUrl(
        this.scene.id,
        z, x, y,
        this.tileSize,
        extra,
      );
    },

    maskUrlFunction([z, x, y]) {
      if (!this.scene) return;
      return getTileUrl(
        this.scene.id,
        z, x, y,
        this.tileSize,
        {
          transparency_mask: true,
        },
      );
    },

    elementToViewportCoordinates(eventOrPoint) {
      if (!this.map) {
        return null;
      }
      const event = eventOrPoint.originalEvent || eventOrPoint;
      const coord = this.map.getEventCoordinate(event);
      return this.viewFromCoordinate(coord);
    },

    zoomFromView(view) {
      if (!view || !this.map) return null;

      const vw = view.w;
      const vh = view.h;
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
      return {
        x: (extent[0] - fullExtent[0]) * sx,
        y: (fullExtent[3] - extent[3]) * sy,
        w: (extent[2] - extent[0]) * sx,
        h: (extent[3] - extent[1]) * sy,
      }
    },

    extentFromView(view) {
      if (!this.scene) throw new Error("Scene not found");
      const fullExtent = this.projection.getExtent();
      const fw = fullExtent[2] - fullExtent[0];
      const fh = fullExtent[3] - fullExtent[1];
      const sx = fw / this.scene.bounds.w;
      const sy = fh / this.scene.bounds.h;
      return [
        fullExtent[0] + view.x * sx,
        fullExtent[3] - (view.y + view.h) * sy,
        fullExtent[0] + (view.x + view.w) * sx,
        fullExtent[3] - view.y * sy,
      ];
    },

    viewFromCoordinate(coord) {
      if (!this.scene) return null;
      const fullExtent = this.projection.getExtent();
      const [xa, ya, xb, yb] = fullExtent;
      return {
        x: (coord[0] - xa) / (xb - xa) * this.scene.bounds.w,
        y: (yb - coord[1]) / (yb - ya) * this.scene.bounds.h,
      }
    },

    elementFromView(view) {
      if (!this.scene) return null;
      const fullExtent = this.projection.getExtent();
      const [xa, ya, xb, yb] = fullExtent;
      const sw = this.scene.bounds.w;
      const sh = this.scene.bounds.h;
      const extent = this.extentFromView(view);
      return {
        x: extent[0] * sw / (xb - xa) + xa,
        y: -extent[1] * sh / (yb - ya) + ya,
        w: extent[2] * sw / (xb - xa) - extent[0] * sw / (xb - xa),
        h: -extent[3] * sh / (yb - ya) + extent[1] * sh / (yb - ya),
      }
    },

    pixelCornersFromView(view) {
      if (!this.map) return null;
      const extent = this.extentFromView(view);
      // Coordinate from extent
      const tl = getTopLeft(extent);
      const tr = getTopRight(extent);
      const bl = getBottomLeft(extent);
      const br = getBottomRight(extent);
      return {
        tl: this.map.getPixelFromCoordinate(tl),
        tr: this.map.getPixelFromCoordinate(tr),
        bl: this.map.getPixelFromCoordinate(bl),
        br: this.map.getPixelFromCoordinate(br),
      }
    },

    setPendingAnimationTime(t) {
      this.pendingAnimationTime = t;
    },

    setPendingTransition(t) {
      this.pendingTransition = t;
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
        const prevZoom = Geoview.fromView(this.latestView, this.scene.bounds)[2];
        const zoom = Geoview.fromView(view, this.scene.bounds)[2];
        const zoomDiff = Math.abs(zoom - prevZoom);

        if (zoomDiff > 1e-4 && !options) {
          const t = Math.max(0.3, Math.pow(zoomDiff, 0.8) * 0.1);
          options = { animationTime: t }
        }
      }

      this.latestView = view;
      
      if (this.pendingAnimationTime && !options) {
        options = { animationTime: this.pendingAnimationTime }
      }
      this.pendingAnimationTime = null;

      if (this.pendingTransition) {
        options = {
          animationTime: this.pendingTransition.t,
          ease: this.pendingTransition.ease,
        }
        const extent = this.extentFromView({
          x: view.x + (this.pendingTransition.x || 0),
          y: view.y + (this.pendingTransition.y || 0),
          w: view.w,
          h: view.h,
        });
        this.v.fit(extent);
      }
      this.pendingTransition = null;

      if (this.v.getAnimating()) {
        this.v.cancelAnimations();
      }

      this.lastAnimationTime = options?.animationTime || 0;      
      
      const fitOpts = options ? {
        duration: (options.animationTime || 0)*1000,
        easing:
          options.ease == "in" ? easeIn :
          options.ease == "out" ? easeOut :
          options.ease == "linear" ? linear :
          zoomEase,
      } : undefined;

      const targetExtent = this.extentFromView(view);
      this.v.fit(targetExtent, fitOpts);
    },

  }
};
</script>

<style scoped>

.container, .tileViewer {
  width: 100%;
  height: 100%;
  -webkit-tap-highlight-color: transparent;
}

.container {
  position: relative;
}

.container.focus {
  background: black;
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
