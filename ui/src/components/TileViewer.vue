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
import { defaults as defaultControls } from 'ol/control';
import {MAC} from 'ol/src/has.js';
import equal from 'fast-deep-equal';
import CrossDragPan from './openlayers/CrossDragPan';

import PhotoSkeleton from './PhotoSkeleton.vue';

import "ol/ol.css";
import { getTileUrl } from '../api';
import Kinetic from 'ol/Kinetic';
import { toLonLat, get as getProjection, fromLonLat } from 'ol/proj';
import { getBottomLeft, getTopLeft, getTopRight, getBottomRight } from 'ol/extent';
import VectorLayer from 'ol/layer/Vector';
import VectorSource from 'ol/source/Vector';
import { Polygon } from 'ol/geom';
import Feature from 'ol/Feature';
import Style from 'ol/style/Style';
import Fill from 'ol/style/Fill';
import { watchEffect } from 'vue';


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
    pannable: Boolean,
    zoomable: Boolean,
    zoomTransition: Boolean,
    kinetic: Boolean,
    tileSize: Number,
    view: Object,
    clipview: Object,
    backgroundColor: String,
    selectTagId: String,
    debug: Object,
    loading: Boolean,
    geo: Boolean,
    geoview: Array,
    viewport: Object,
  },

  emits: [
    "zoom",
    "click",
    "pointer-down",
    "view",
    "geoview",
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
      clipviewZoom: 1,
    }
  },
  async created() {
  },
  async mounted() {
    this.latestView = null;
    this.lastClipview = null;
    this.clipviewChangeTime = 0;
    this.lastAnimationTime = 0;
    this.reset();
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

    pannable: {
      immediate: true,
      handler(newValue) {
        this.dragPan?.setActive(newValue);
      }
    },

    zoomable: {
      immediate: true,
      handler(newValue) {
        this.mouseWheelZoom?.setActive(newValue);
      }
    },

    geoview: {
      immediate: true,
      handler(geoview) {
        if (!geoview) return;
        if (
          this.lastGeoview &&
          Math.abs(geoview[0] - this.lastGeoview[0]) < 1e-4 &&
          Math.abs(geoview[1] - this.lastGeoview[1]) < 1e-4 &&
          Math.abs(geoview[2] - this.lastGeoview[2]) < 1e-1
        ) {
          // Geoview is already close enough, nothing to do.
          // This usually happens after the geoview is applied
          // to the url and then the url is read back.
          return;
        }
        this.setGeoview(geoview);
      }
    },

    kinetic(kinetic) {
      this.setKinetic(kinetic);
    },

    view(view) {
      this.setView(view);
    },

    clipview() {
      this.clipviewChangeTime = Date.now();
      // this.setCrossPanActive(!!this.clipview);
    },

    crossPanActive: {
      immediate: true,
      handler(active) {
        // console.log("crossPanActive", active)
        this.crossPan?.setActive(active);
      }
    },

    dragPanActive: {
      immediate: true,
      handler(active) {
        // console.log("dragPanActive", active)
        this.dragPan?.setActive(active);
      }
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
      return [0, 0, width, height];
    },
    viewExtent() {
      let { width, height } = this.getTiledImageSizeAtZoom(this.maxZoom);
      return [0, -height*2, width, height];
    },
    crossPanActive() {
      return this.clipview && this.clipviewZoom < 1.1;
    },
    dragPanActive() {
      return this.pannable && !this.crossPanActive;
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

      
      main.on("prerender", event => {
        // Clip to the top left square
        const ctx = event.context;
        const size = this.map.getSize();
        // const view = this.map.getView();
        // const extent = view.calculateExtent(size);
        const view = this.clipview;
        if (!view) return;
        
        // const corners = this.pixelCornersFromView(view);
        // const pixelRatio = window.devicePixelRatio;
        // corners.tl[0] *= pixelRatio;
        // corners.tl[1] *= pixelRatio;
        // corners.tr[0] *= pixelRatio;
        // corners.tr[1] *= pixelRatio;
        // corners.br[0] *= pixelRatio;
        // corners.br[1] *= pixelRatio;
        // corners.bl[0] *= pixelRatio;
        // corners.bl[1] *= pixelRatio;

        // ctx.save();
        // ctx.rect(
        //   corners.tl[0],
        //   corners.tl[1],
        //   corners.tr[0] - corners.tl[0],
        //   corners.bl[1] - corners.tl[1],
        // );
        // ctx.clip();

        // ctx.strokeStyle = "green";
        // ctx.lineWidth = 20;
        // // ctx.beginPath();
        
        // ctx.strokeRect(
        //   corners.tl[0],
        //   corners.tl[1],
        //   corners.tr[0] - corners.tl[0],
        //   corners.bl[1] - corners.tl[1],
        // );

        // Highlight the view instead
      });

      main.on("postrender", event => {
        const ctx = event.context;
        const view = this.clipview;
        if (!view) return;

        // ctx.restore();
        
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

        const animFrac = 
          this.lastAnimationTime ?
            Math.max(0, Math.min(1, (Date.now() - this.clipviewChangeTime) / (this.lastAnimationTime*1000))) :
            1;
        
        const latestZoom = this.zoomFromView(this.latestView);
        const zoom = this.zoomFromView(view);
        const zoomFrac = Math.min(1, latestZoom/zoom + 1e-4);
        
        const mapExtent = this.v.getProjection().getExtent();
        const mapRes = this.v.getResolutionForExtent([
          0,
          0,
          mapExtent[2],
          mapExtent[2],
        ]);
        const viewExtent = this.extentFromView(view);
        const viewRes = this.v.getResolutionForExtent(viewExtent);
        const res = this.v.getResolution();
        const resFrac = 1 - Math.min(1, (res - viewRes) / (mapRes - viewRes));
        // console.log(resFrac)

        // const alpha = animFrac * resFrac;
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

      // const clip = new VectorLayer({
      //   source: new VectorSource({
      //     features: [
      //       new Feature({
      //         geometry: new Polygon([
      //           [
      //             [0, 0],
      //             [0, 5.5e11],
      //             [2e11, 2e11],
      //             [2e11, 0],
      //             [0, 0],
      //           ],
      //         ]),
      //       }),
      //     ],
      //   }),
      //   style: new Style({
      //     fill: new Fill({
      //       color: "rgba(0, 0, 0, 0.5)",
      //     }),
      //   }),
      // });

      if (this.geo) {

        const mask = new TileLayer({
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
          // clip,
        ];
      }
    },

    initOpenLayers(element) {

      if (this.geo) {
        this.projection = getProjection("EPSG:3857");
      } else {
        this.projection = new Projection({
          code: "tiles",
          units: "pixels",
          extent: this.projectionExtent,
        });
      }

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
      const crossPan = new CrossDragPan();
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
        this.v = new View({
          projection: this.projection,
          center: this.pendingGeoview ?
            this.geoviewToCenter(this.pendingGeoview) :
            [0, 0],
          zoom: this.pendingGeoview ?
            this.geoviewToZoom(this.pendingGeoview) :
            2,
          enableRotation: false,
        });
      } else {
        const extent = this.viewExtent;
        this.v = new View({
          center: [extent[2]/2, extent[3]],
          projection: this.projection,
          zoom: 0,
          minZoom: 0,
          maxZoom: this.maxZoom,
          enableRotation: false,
          extent,
          smoothExtentConstraint: false,
          showFullExtent: true,
          constrainOnlyCenter: true,
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
      // console.log("moveend", view)
      if (this.clipview) {
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
      if (this.geo) {
        const geoview = this.getGeoview();
        this.lastGeoview = geoview;
        this.$emit("geoview", geoview);
      }
    },

    onResolutionChange(event) {
      const visibleExtent = this.v.calculateExtent(this.map.getSize());
      const view = this.viewFromExtent(visibleExtent);
      if (!view) return;
      this.latestView = view;
      this.$emit("view", view);
      if (this.geo) {
        const geoview = this.getGeoview();
        this.lastGeoview = geoview;
        this.$emit("geoview", geoview);
      }
      if (this.clipview) {
        const clipzoom = this.zoomFromView(this.clipview);
        const viewzoom = this.zoomFromView(view);
        const ratio = viewzoom / clipzoom;
        this.clipviewZoom = ratio;
      }
      // if (this.clipview && !this.crossPan.axis) {
      //   const resDiff = event.target.get(event.key) - event.oldValue;
      //   if (resDiff > 0) {
      //     const clipzoom = this.zoomFromView(this.clipview);
      //     const viewzoom = this.zoomFromView(view);
      //     const ratio = viewzoom / clipzoom;
      //     if (ratio < 0.8) {
      //       this.$emit("nav", {
      //         x: 0,
      //         y: -1,
      //       })
      //     }
      //   }
      // }
      // console.log("resolution change", this.zoomFromView(this.clipview), this.zoomFromView(view), this.zoomFromView(view) / this.zoomFromView(this.clipview), this.crossPan.panning)
      // if (this.resolutionChangeTimer) {
      //   clearTimeout(this.resolutionChangeTimer);
      // }
      // this.resolutionChangeTimer = setTimeout(() => {
      //   this.resolutionChangeTimer = null;
      //   this.$emit("move-end", view);
      // }, 100);
    },

    onBoxSelect(event, extent) {
      const shift = event.mapBrowserEvent.originalEvent.shiftKey;
      const view = this.viewFromExtent(extent);
      if (!view) return;
      this.$emit("box-select", view, shift);
    },

    onNav(event) {
      if (event.interrupted) {
        this.navOnZoom(event);
        return;
      }
      this.$emit("nav", event);
    },

    navOnZoom() {
      const ratio = this.clipviewZoom;
      if (ratio < 0.6) {
        this.$emit("nav", { x: 0, y: -1 });
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
      if (this.selectTagId) {
        extra.select_tag = this.selectTagId;
      }
      return getTileUrl(
        this.scene.id,
        z, x, y,
        this.tileSize,
        this.backgroundColor,
        extra,
      );
    },

    maskUrlFunction([z, x, y]) {
      if (!this.scene) return;
      return getTileUrl(
        this.scene.id,
        z, x, y,
        this.tileSize,
        null,
        {
          transparency_mask: true,
        },
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
      if (!view || !this.map) return null;
      const vw = view.w;
      const vh = view.h;
      // const sw = this.scene.bounds.w;
      // const sh = this.scene.bounds.h;
      const [mw, mh] = this.map.getSize();
      const zw = mw / vw;
      const zh = mh / vh;

      // func (rect Rect) FitInside(container Rect) (out Rect) {
      // 	imageRatio := rect.W / rect.H

      // 	var scale float64
      // 	if container.W/container.H < imageRatio {
      // 		scale = container.W / rect.W
      // 	} else {
      // 		scale = container.H / rect.H
      // 	}

      // 	out.W = rect.W * scale
      // 	out.H = rect.H * scale
      // 	out.X = container.X + (container.W-out.W)*0.5
      // 	out.Y = container.Y + (container.H-out.H)*0.5
      // 	return out
      // }

      // const viewAspect = vw / vh;
      // const mapAspect = mw / mh;
      // const mapScale = mapAspect < viewAspect ? mw / vw : mh / vh;

      // const sceneAspect = sw / sh;
      // const sceneScale = sceneAspect < 1 ? sw / vw : sh / vh;

      // const viewport = this.viewport;
      // const [vpw, vph] = [viewport.width.value, viewport.height.value];
      // const vpAspect = vpw / vph;
      // const vpScale = vpAspect < viewAspect ? vpw / vw : vph / vh;

      // // console.log({ vw, vh, sw, sh, zw, zh, min: Math.min(zw, zh), scale })
      // console.log({ min: Math.min(zw, zh), mapScale, sceneScale, vpScale })
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

    getGeoview() {
      const center = toLonLat(this.v.getCenter());
      const zoom = this.v.getZoom();
      return [center[0], center[1], zoom];
    },

    setGeoview(geoview) {
      this.lastGeoview = geoview;
      if (!this.map) {
        console.info("Map not initialized yet, setting pending geoview", geoview);
        this.pendingGeoview = geoview;
        return;
      }
      this.v.setCenter(this.geoviewToCenter(geoview));
      this.v.setZoom(this.geoviewToZoom(geoview));
    },

    geoviewToCenter(geoview) {
      return fromLonLat(geoview.slice(0, 2));
    },

    geoviewToZoom(geoview) {
      return geoview[2];
    },

    // setCrossPanActive(active) {
    //   // this.dragPan.setActive(this.pannable && !active);
    //   // this.crossPan.setActive(active);
    //   // Initially false
    //   // on clipview this.setCrossPanActive(!!this.clipview);
      
    // },

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

        const skipClipTransition =
          this.lastClipview &&
          this.latestView.x == this.lastClipview.x &&
          this.latestView.y == this.lastClipview.y &&
          this.latestView.w == this.lastClipview.w &&
          this.latestView.h == this.lastClipview.h;
          
        if (zoomDiff > 1e-4 && !options && !skipClipTransition) {
          // console.log(zoomDiff)
          // const t = zoomDiff * 0.05;
          // const t = zoomDiff * 0.2;
          // const t = zoomDiff * 1;
          const t = Math.pow(zoomDiff, 0.5) * 0.08;
          // const t = Math.pow(zoomDiff, 1.5) * 0.08;
          options = { animationTime: t }
        }
      }

      this.latestView = view;
      this.lastClipview = this.clipview;
      if (this.pendingAnimationTime && !options) {
        options = { animationTime: this.pendingAnimationTime }
      }
      this.pendingAnimationTime = null;

      if (this.v.getAnimating()) {
        this.v.cancelAnimations();
      }

      const targetExtent = this.extentFromView(view);

      this.lastAnimationTime = options?.animationTime || 0;
      
      const fitOpts = options ? {
        duration: options.animationTime*1000,
        easing: function(x) {
          return x < 0.5 ? 4 * x * x * x : 1 - Math.pow(-2 * x + 2, 3) / 2
        },
        // easing: function(x) {
        //   return x * x * x * (x * (6.0 * x - 15.0) + 10.0);
        // },
        // easing: function(t) {
        //   return Math.pow(t, 0.5) + 0.3;
        // },
        // easing: function(t) {
        //   return Math.pow(2, -50 * t) * Math.sin(((t - 0.1) * (2 * Math.PI)) / 0.3) + 1
        // },
        // easing: function(t) {
        //   return Math.pow(2, -80 * t) * Math.sin(((t - 0.1) * (2 * Math.PI)) / 0.3) + 1
        // },
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
  -webkit-tap-highlight-color: transparent;
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
