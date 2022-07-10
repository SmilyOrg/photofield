<template>
  <div class="container" :class="{ fullpage, fixed: !nativeScroll }">

    <page-title :title="pageTitle"></page-title>

    <tile-viewer
      class="viewer"
      ref="viewer"
      :interactive="!nativeScroll && !panDisabled"
      :scene="scene"
      :tileSize="tileSize"
      :debug="settings.debug"
      @viewer="overlayViewer = $event"
      @zoom="onZoom"
      @view="onView"
      @click="onClick"
      @load="onLoad"
      @keydown="onKeyDown"
      @contextmenu.prevent="onContext"
    ></tile-viewer>
    
    <overlays
      :viewer="overlayViewer"
      :overlay="lastViewedRegion"
      :scene="scene"
      :active="!nativeScroll"
      @interactive="interactive => panDisabled = interactive"
      class="overlays"
    ></overlays>

    <spinner
      class="spinner"
      :total="scene?.file_count"
      :speed="sceneLoadFilesPerSecond"
      :divider="10000"
      :loading="scene?.loading"
    ></spinner>
    
    <div
      class="scroller"
      :class="{ disabled: !nativeScroll }"
      ref="scroller"
      @pointerdown="onPointerDown"
      @pointerup="onPointerUp"
      @touchStart="onTouchStart"
      @touchEnd="onTouchEnd"
      @wheel="onWheel"
      @scroll="onScroll"
      @contextmenu.prevent="onContext"
    >
      <div
        class="virtual-canvas"
        ref="virtualCanvas"
        :style="{ height: canvas.height + 'px' }">
      </div>
    </div>
    
    <ContextMenu
      class="context-menu"
      ref="contextMenu"
    >
      <region-menu
        :region="contextRegion"
        :scene="scene"
        :sceneParams="sceneParams"
        :flipX="contextFlipX"
        :flipY="contextFlipY"
        :tileSize="tileSize"
        @close="closeContextMenu()"
      ></region-menu>
    </ContextMenu>
  </div>
</template>

<script>
import { computed, nextTick, ref, toRef, watch, watchEffect } from 'vue';
import qs from "qs";
import { isCloseClick } from '../utils';
import TileViewer from './TileViewer.vue';
import { createScene, getRegions, useApi } from '../api';
import { timeout, useTask, useTaskGroup } from "vue-concurrency";
import PageTitle from './PageTitle.vue';
import Simulation from '../simulation';
import ContextMenu from '@overcoder/vue-context-menu';
import RegionMenu from './RegionMenu.vue';
import dateParseISO from 'date-fns/parseISO';
import dateFormat from 'date-fns/format';
import differenceInDays from 'date-fns/differenceInDays';
import Overlays from './Overlays.vue';
import Spinner from './Spinner.vue';

export default {

  props: [
    "collectionId",
    "regionId",
    "settings",
    "fullpage",
    "scrollbar",
  ],

  emits: {
    load: null,
    tasks: null,
    immersive: immersive => typeof immersive == "boolean",
    scene: null,
    reindex: null,
  },

  components: {
    TileViewer,
    PageTitle,
    ContextMenu,
    RegionMenu,
    Overlays,
    Spinner,
  },

  data() {
    return {
      tileSize: 512,
      marginTop: 64,
      loadProgress: 0,
      view: {
        x: 0,
        y: 0,
        width: 0,
        height: 0,
      },
      contextRegion: null,
      contextAnchor: "",
      contextFlipX: false,
      contextFlipY: false,
      contextMenuOpen: false,
      overlayViewer: null,
      lastViewedRegion: null,
      panDisabled: false,
    }
  },

  setup(props, { emit }) {
    const collectionId = toRef(props, "collectionId");
    const regionId = toRef(props, "regionId");
    const scrollbar = toRef(props, "scrollbar");
    const sceneLoadFilesPerSecond = ref(0);

    const nativeScroll = ref(true);

    const window = ref({
      x: 0,
      y: 0,
      width: 0,
      height: 0,
    });

    const {
      data: collection,
    } = useApi(() => collectionId && `/collections/${collectionId.value}`);

    watch(collection, async newValue => {
      if (!newValue) return;
      let autoIndex = false;
      if (!newValue.indexed_at) {
        console.log(`not indexed yet, indexing...`);
        autoIndex = true;
      } else {
        const indexedAt = dateParseISO(newValue.indexed_at);
        const now = new Date();
        const days = differenceInDays(now, indexedAt);
        if (days >= 1) {
          console.log(`indexed ${days} days ago, reindexing...`);
          autoIndex = true;
        }
      }
      if (autoIndex) {
        emit("reindex");
      }
    })

    const sceneParams = computed(() =>
      window?.value?.width &&
      {
        collection_id: collectionId.value,
        image_height: props.settings.image.height,
        scene_width: window.value.width,
        layout: props.settings.layout,
      }
    );
    
    const {
      items: scenes,
      isValidating: scenesLoading,
      itemsMutate: scenesMutate,
    } = useApi(() => sceneParams.value && `/scenes?` + qs.stringify(sceneParams.value));

    const recreateScenesInProgress = ref(0);
    const recreateScene = async () => {
      recreateScenesInProgress.value = recreateScenesInProgress.value + 1;
      const params = sceneParams.value;
      await scenesMutate(async () => ([await createScene(params)]));
      recreateScenesInProgress.value = recreateScenesInProgress.value - 1;
    }

    watch(scenes, async newValue => {
      // Create scene if a matching one hasn't been found
      if (newValue?.length === 0) {
        console.log("scene not found, creating...");
        await recreateScene();
      }
    })

    const scene = computed(() => {
      const list = scenes?.value;
      if (!list || list.length == 0) return null;
      return list[0];
    });
    watch(scene, async (newValue, oldValue) => {
      if (newValue?.loading) {
        let prev = oldValue?.file_count || 0;
        if (prev > newValue.file_count) {
          prev = 0;
        }
        sceneLoadFilesPerSecond.value = newValue.file_count - prev;
        sceneRefreshTask.perform();
      } else {
        sceneLoadFilesPerSecond.value = 0;
      }
    })
    const sceneRefreshTask = useTask(function*() {
      yield timeout(1000);
      scenesMutate();
    }).keepLatest()


    const regionSeekId = ref(null);
    const regionSeekApplyTask = useTask(function*(_, router) {
      yield timeout(1000);

      const seekId = regionSeekId.value;
      router.push({
        name: "region",
        params: {
          collectionId: collectionId.value,
          regionId: seekId,
        },
      });
    }).restartable();

    const reorientPending = ref(null);
    const resizeApplyTask = useTask(function*(_, rect) {
      scrollbar.value?.sleep();

      yield timeout(100);

      const pre = visibleView?.value;
      let preCenterRegionId = null;

      // Find closest region to center
      if (pre && pre.w != 0 && pre.h != 0) {
        const regions = yield getRegions(
          scene.value.id,
          pre.x,
          pre.y,
          pre.w,
          pre.h,
        );
        const precx = pre.x + pre.w/2;
        const precy = pre.y + pre.h/2;
        let minDistSq = Infinity;
        for (let i = 0; i < regions.length; i++) {
          const region = regions[i];
          const rcx = region.bounds.x + region.bounds.w/2;
          const rcy = region.bounds.y + region.bounds.h/2;
          const dx = rcx - precx;
          const dy = rcy - precy;
          const distSq = dx*dx + dy*dy;
          if (distSq < minDistSq) {
            minDistSq = distSq;
            preCenterRegionId = region.id;
          }
        }
      }

      if (preCenterRegionId) {
        reorientPending.value = {
          regionId: preCenterRegionId,
          rect,
        }
      } else if (!reorientPending.value) {
        scrollbar.value?.update();
      }

      window.value = rect;
    }).restartable();

    const { data: reorientRegion } = useApi(() => {
      return reorientPending?.value &&
      scene?.value?.id &&
      reorientPending.value.rect.width === scene?.value?.bounds?.w &&
      `/scenes/${scene.value.id}/regions/${reorientPending.value.regionId}`
    });

    const scrollbarUpdateRegion = ref(null);
    watch(reorientRegion, reorientRegion => {
      if (reorientRegion) {
        scrollbarUpdateRegion.value = reorientRegion;
        reorientRegion.value = null;
        scrollbar.value?.update();
      }
    })

    const { data: region } = useApi(() => 
      scene?.value?.id &&
      (regionSeekId?.value || regionId?.value) &&
      `/scenes/${scene.value.id}/regions/${regionSeekId?.value || regionId.value}`
    );

    const visibleView = ref(null);

    const visibleRegionsTask = useTask(function*(_, view, sceneParams) {
      yield timeout(200);
      visibleView.value = view;
    }).keepLatest();

    const {
      items: visibleRegions,
    } = useApi(() =>
      scene?.value?.id &&
      visibleView?.value &&
      `/scenes/${scene.value.id}/regions?${qs.stringify({
        ...visibleView.value,
        limit: 1,
      })}`
    );

    const scrollbarLabel = computed(() => {
      if (visibleRegions?.value?.length > 0) {
        const region = visibleRegions.value[0];
        const date = dateParseISO(region.data?.created_at);
        if (!date) return;
        return dateFormat(date, "d MMM yyyy");
      }
    })

    watch([scenesLoading, recreateScenesInProgress], ([scenesLoading, recreatingCount]) => {
      const tasks = [];
      let count = 0;
      if (scenesLoading) count++;
      count += recreatingCount;
      if (count > 0) {
        tasks.push({
          id: "scene-load",
          name: "Loading scene",
          pending: count,
        });
      }
      emit("tasks", tasks);
    })
    
    return {
      nativeScroll,
      scrollbarUpdateRegion,
      reorientRegion,
      recreateScene,
      window,
      resizeApplyTask,
      collection,
      scene,
      scenes,
      sceneLoadFilesPerSecond,
      region,
      regionSeekId,
      regionSeekApplyTask,
      visibleRegionsTask,
      visibleRegions,
      scrollbarLabel,
    }
  },

  async mounted() {
    if (this.regionId != null) {
      this.regionFocusPending = true;
      this.nativeScroll = false;
    }
    this.addResizeObserver();
    // this.$refs.scroller.addEventListener("scroll", this.onScroll);
    this.$bus.on("home", this.navigateExit);
    this.$bus.on("recreate-scene", this.recreateScene);
    this.$bus.on("simulate-run", this.simulate);
    if (this.fullpage) this.addFullpageListeners();
    // this.simulate();
  },
  unmounted() {
    clearInterval(this.demoInterval);
    this.removeResizeObserver();
    this.$bus.off("home", this.navigateExit);
    this.$bus.off("recreate-scene", this.recreateScene);
    this.$bus.off("simulate-run", this.simulate);
    if (this.fullpage) this.removeFullpageListeners();
  },
  computed: {
    pageTitle() {
      if (!this.collection) {
        return "Photos";
      }
      const regionId = this.regionSeekId || this.regionId;
      if (this.regionId == null) {
        return `${this.collection.name} - Photos`;
      }
      return `#${regionId} - ${this.collection.name} - Photos`;
    },
    viewer() {
      return {
        scene: {
          width: this.scene?.bounds.w || 0,
          height: this.scene?.bounds.h || 0,
          params: this.sceneParams,
        }
      }
    },
    sceneParams() {
      const params = {
        collection: this.collectionId,
        imageHeight: this.settings.image.height,
        sceneWidth: this.window.width,
        layout: this.settings.layout == "default" ? undefined : this.settings.layout,
      };
      if (params.collection == null) {
        return null;
      }
      if (params.sceneWidth == 0) {
        return null;
      }
      return Object.entries(params).map(([key, value]) => `${key}=${value}`).join("&");
    },
    canvas() {
      const aspectRatio =
        this.scene?.bounds?.h ?
          this.scene.bounds.w / this.scene.bounds.h :
          1;
      return {
        width: this.window.width,
        height: this.window.width / aspectRatio,
      }
    },
    pointerDistThreshold() {
      return this.$refs.viewer.pointerDistThreshold;
    },
    pointerTimeThreshold() {
      return this.$refs.viewer.pointerTimeThreshold;
    },
  },
  watch: {
    scene(newScene, oldScene) {
      if (
        oldScene && newScene &&
        oldScene.id == newScene.id &&
        oldScene.file_count == newScene.file_count
      ) {
        return;
      }
      this.$emit("scene", newScene);
      if (newScene) {
        this.pushScrollToView();
      }
    },
    fullpage(fullpage) {
      if (fullpage) {
        addFullpageListeners();
      } else {
        removeFullpageListeners();
      }
    },
    scrollbar: {
      immediate: true,
      handler(newScrollbar, oldScrollbar) {
        this.detachScrollbar(oldScrollbar);
        this.attachScrollbar(newScrollbar);
      },
    },
    scrollbarLabel: {
      immediate: true,
      handler(label) {
        this.scrollbarHandle?.setLabel(label);
      },
    },
    regionId: {
      immediate: true,
      async handler(regionId) {
        this.regionSeekId = null;
        // console.log("regionId", regionId)
        if (this.regionId != null && !this.wasRecentlyFocused()) {
          this.nativeScroll = false;
          this.regionFocusPending = true;
        }
      },
    },
    region(region) {
      // console.log("region", this.regionFocusPending, this.region)
      if (this.region && this.regionFocusPending) {
        this.viewRegion(this.region);
        this.regionFocusPending = null;
        this.scrollbarUpdateRegion = this.region;
      }
    },
    nativeScroll: {
      immediate: true,
      handler(nativeScroll) {
        this.$emit("immersive", !nativeScroll);
      },
    }
  },
  methods: {

    addResizeObserver() {
      this.removeResizeObserver();
      this.resizeObserver = new ResizeObserver(entries => {
        this.onResize(entries[0].contentRect);
      });
      this.resizeObserver.observe(this.$refs.viewer.$el);
    },

    removeResizeObserver() {
      if (this.resizeObserver) {
        this.resizeObserver.disconnect();
        this.resizeObserver = null;
      }
    },

    addFullpageListeners() {
      window.addEventListener('scroll', this.onScroll);
      window.addEventListener('resize', this.onWindowResize);
    },

    removeFullpageListeners() {
      window.removeEventListener('scroll', this.onScroll);
      window.removeEventListener('resize', this.onWindowResize);
    },

    attachScrollbar(scrollbar) {
      if (!scrollbar) return;
      scrollbar.options({
        callbacks: {
          onScroll: this.onScroll,
          onUpdated: this.onScrollbarUpdated,
        },
      });
      this.scrollbarHandle = scrollbar.ext("timeline");
    },

    detachScrollbar(scrollbar) {
      if (!scrollbar) return;
      scrollbar.options({
        callbacks: {
          onScroll: null,
        },
      });
      this.scrollbarHandle = null;
    },
    
    wasRecentlyFocused() {
      return this.focusRegionTime !== undefined && Date.now() - this.focusRegionTime < 200;
    },
    
    async simulate() {
      this.navigateExit();

      const tileEvaluation = {
        runs: [
          { tileSize: 50 },
          { tileSize: 100 },
          { tileSize: 150 },
          { tileSize: 200 },
          { tileSize: 250 },
          { tileSize: 300 },
          { tileSize: 350 },
          { tileSize: 400 },
          { tileSize: 450 },
          { tileSize: 500 },
          { tileSize: 550 },
          { tileSize: 600 },
          { tileSize: 650 },
          { tileSize: 700 },
          { tileSize: 750 },
          { tileSize: 800 },
          { tileSize: 850 },
          { tileSize: 900 },
          { tileSize: 950 },
          { tileSize: 1000 },
          { tileSize: 1050 },
          { tileSize: 1100 },
          { tileSize: 1150 },
          { tileSize: 1200 },
          { tileSize: 1250 },
          { tileSize: 1300 },
        ],
        actions: [
          { duration: 500, scroll: { from: 1000-10 } },
          { duration: 1000, scroll: { from: 1000 } },
          { duration: 5000, scroll: { from: 1000, to: 2000 } },
          { duration: 5000, scroll: { from: 2000, to: 12000  } },
          { duration: 5000, scroll: { from: 12000, to: 62000  } },
          { duration: 3000 },
        ],
      }

      const stopAndGo = {
        runs: [{ tileSize: this.tileSize }],
        actions: [
          { duration: 500, scroll: { from: 1000-10 } },
          { duration: 1000, scroll: { from: 1000 } },
          { duration: 5000, scroll: { from: 1000, to: 2000 } },
          { duration: 5000, scroll: { from: 2000, to: 12000 } },
          { duration: 5000, scroll: { from: 12000, to: 62000 } },
          { duration: 3000 },
          { duration: 3000, scroll: { from: 62000, to: 100000 } },
          { duration: 500 },
          { duration: 2000, scroll: { from: 100000, to: 12000 } },
        ]
      }

      const fast = {
        runs: [{ tileSize: this.tileSize }],
        actions: [
          { duration: 500, scroll: { from: 1000-10 } },
          { duration: 1000, scroll: { from: 1000 } },
          { duration: 5000, scroll: { from: 1000, to: 50000 } },
          { duration: 4000, scroll: { from: 50000, to: 1000 } },
          { duration: 3000, scroll: { from: 1000, to: 50000 } },
          { duration: 2000, scroll: { from: 50000, to: 1000 } },
          { duration: 1000, scroll: { from: 1000, to: 50000 } },
          { duration: 750, scroll: { from: 50000, to: 1000 } },
          { duration: 500, scroll: { from: 1000, to: 50000 } },
          { duration: 250, scroll: { from: 50000, to: 1000 } },
          { duration: 100, scroll: { from: 1000, to: 50000 } },
          { duration: 50, scroll: { from: 50000, to: 1000 } },
        ]
      }

      this.simulation = new Simulation({
        ...fast,
        scrollbar: this.scrollbar,
      });
      const results = await this.simulation.run(this);
      console.log(JSON.stringify(results, null, 2));
      this.$bus.emit("simulate-done");
    },

    demoScroll() {
      const y = (1 + Math.sin(Date.now() * Math.PI * 2 / 1000 * 0.05)) / 2 * (this.scene.bounds.h - this.window.height);
      this.$refs.scroller.scroll(0, y);
      window.requestAnimationFrame(this.demoScroll);
    },

    onResize(rect) {
      if (rect.width == 0 || rect.height == 0) return;
      this.resizeApplyTask.perform(rect, this.pushScrollToView);
    },

    onWindowResize(rect) {
      if (rect.width == 0 || rect.height == 0) return;
      const vh = window.innerHeight * 0.01;
      document.documentElement.style.setProperty('--vh', `${vh}px`);
    },

    onScroll(event) {
      if (!this.nativeScroll) return;
      this.closeContextMenu();
      this.pushScrollToView();
    },

    onScrollbarUpdated() {
      if (this.scrollbarUpdateRegion) {
        this.pushViewToScroll(this.scrollbarUpdateRegion.bounds);
        this.scrollbarUpdateRegion = null;
      }
    },

    onLoad(event) {
      this.loadProgress = event.inProgress / event.limit;
      this.$emit("load", { image: event });
    },

    redispatchEventToViewer(event) {
      const target = this.$refs.viewer.pointerTarget;
      const redirected = new event.constructor(event.type, event);
      target.dispatchEvent(redirected);
    },

    async onWheel(event) {
      if (event.ctrlKey && this.nativeScroll) {
        event.preventDefault();
        if (event.deltaY < 0) {
          this.nativeScroll = false;
          await nextTick();
          this.redispatchEventToViewer(event);
        }
      }
    },

    async onPointerDown(event) {
      if (!this.nativeScroll) {
        return;
      }
      if (event.button != 0) {
        return;
      }
      this.lastPointerDownEvent = event;
      // console.log("DOWN", event);
    },

    async onPointerUp(event) {
      if (!this.nativeScroll) {
        return;
      }
      if (event.button != 0) {
        return;
      }
      if (!this.lastPointerDownEvent) {
        return;
      }
      this.lastPointerUpEvent = event;
      if (this.contextMenuOpen) {
        this.closeContextMenu();
        return;
      }
      // console.log("UP", event);
      const down = this.lastPointerDownEvent;
      const up = this.lastPointerUpEvent;
      const close = isCloseClick(
        down,
        up,
        this.pointerTimeThreshold,
        this.pointerDistThreshold
      );
      if (close) {
        const zoomClick = await this.onClick(this.$refs.viewer.elementToViewportCoordinates(down));
        if (zoomClick) {
          this.nativeScroll = false;
        }
        // console.log("redisp", down, up)
        // this.redispatchEventToViewer(down);
        // this.redispatchEventToViewer(up);
      }
    },

    async onTouchStart(event) {
    //   if (!this.nativeScroll) {
    //     return;
    //   }
    //   console.log("TOUCH START", event);
    //   if (this.nativeScroll && event.touches.length >= 2) {
    //     this.nativeScroll = false;
    //     await nextTick();
    //     this.redispatchEventToViewer(this.lastTouchStartEvent);
    //     this.redispatchEventToViewer(event);
    //  }
    //   this.lastTouchStartEvent = event;
    },

    async onTouchEnd(event) {
      // if (!this.nativeScroll) {
      //   return;
      // }
      // console.log("TOUCH END", event);
      // this.lastTouchStartEvent = null;
    },

    async onZoom(zoom) {
      if (!this.wasRecentlyFocused() && zoom < 0.99) {
        this.nativeScroll = true;
        this.pushScrollToView();
      }
    },

    async onPan(view) {
      if (this.nativeScroll) return;
      this.pushViewToScroll(view);
    },

    async onView(view) {
      this.view = view;
      if (this.nativeScroll) return;
      this.pushViewToScroll(view);
    },

    async onClick(event) {
      const regions = await getRegions(this.scene.id, event.x, event.y, 0, 0);
      if (regions && regions.length > 0) {
        const region = regions[0];
        // console.log(region);

        const viewerArea = this.view.w * this.view.h;
        const regionArea = region.bounds.w * region.bounds.h;
        const areaDiff = viewerArea/regionArea;
        // const animationTime = Math.abs(Math.log(areaDiff) / 2);
        // const animationTime = Math.pow(areaDiff, 0.2);
        const animationTime = Math.pow(areaDiff, 0.4)*0.08;
        // console.log(viewerArea, regionArea, areaDiff, animationTime)
        
        this.focusRegion(region, animationTime);
        return true;
      }
      return false;
    },

    async onContext(event) {
      this.contextRegion = null;
      this.openContextMenu(event);
      const menuWidth = 250;
      const menuHeight = 300;
      const right = event.x + menuWidth;
      const bottom = event.y + menuHeight;
      this.contextFlipX = right > window.innerWidth;
      this.contextFlipY = bottom > window.innerHeight;
      const pos = this.$refs.viewer.elementToViewportCoordinates(event);
      const regions = await getRegions(this.scene?.id, pos.x, pos.y, 0, 0);
      if (regions && regions.length > 0) {
        this.contextRegion = regions[0];
      }
    },

    openContextMenu(event) {
      this.$refs.contextMenu.open(event);
      this.contextMenuOpen = true;
    },

    closeContextMenu() {
      if (!this.contextMenuOpen) return;
      this.contextMenuOpen = false;
      this.$refs.contextMenu.close();
    },

    async focusRegion(region, transition) {
      this.focusRegionTime = Date.now();
      this.viewRegion(region, transition);
      this.$router.push({
        name: "region",
        params: {
          collectionId: this.collection.id,
          regionId: region?.id,
        },
      })
    },

    async viewRegion(region, transition) {
      if (!this.lastViewedRegion || this.lastViewedRegion.id != region.id) {
        this.lastViewedRegion = region;
      }
      this.setView(region.bounds, transition);
    },

    setView(view, transition) {
      // console.log(view, transition);
      this.$refs.viewer.setView(
        view,
        transition && { animationTime: transition }
      )
      this.view = view;
      this.pushViewToScroll(view);
    },

    onKeyDown(event) {
      if (this.nativeScroll) return;
      switch (event.key) {
        case "ArrowLeft": this.navigate(-1); return;
        case "ArrowRight": this.navigate(1); return;
        case "Escape": this.navigateExit(); return;
      }
    },

    async navigate(offset) {
      let prevId;
      if (this.regionSeekId != null) {
        prevId = this.regionSeekId;
      } else {
        prevId = parseInt(this.regionId, 10);
      }
      const nextId = prevId + offset;
      if (nextId < 0 || nextId >= this.scene.file_count-1) {
        return;
      }
      this.regionFocusPending = true;
      this.regionSeekId = nextId;
      this.regionSeekApplyTask.perform(this.$router);
    },

    navigateExit() {
      this.nativeScroll = true;
      this.pushScrollToView(null, 1);
      // Firefox fires an onScroll event when you make the scrollbar
      // visible again, which breaks the smooth transition. This ignores scroll
      // events until a short time after exiting navigation to prevent this.
      // this.ignoreScrollToViewUntil = Date.now() + 200;
      this.$router.push({
        name: "collection",
        params: {
          collectionId: this.collectionId,
        },
      });
    },

    pushViewToScroll(view) {
      if (this.nativeScroll) {
        console.warn("Pushing view to scroll while in native scrolling mode");
      }

      if (!this.scene?.bounds?.h) {
        console.warn("Scene has zero height, view to scroll pending", view);
        this.pendingViewToScroll = view;
        return;
      }

      const scroller = this.$refs.scroller;
      let scrollMaxY;
      if (this.scrollbar) {
        if (!scroller) {
          console.warn("Scroller not found, view to scroll pending", view);
          this.pendingViewToScroll = view;
          return;
        }
        scrollMaxY = scroller.scrollHeight - scroller.clientHeight;
      } else if (this.fullpage) {
        scrollMaxY = document.body.scrollHeight - window.innerHeight;
      } else {
        if (!scroller) {
          console.warn("Scroller not found, view to scroll pending", view);
          this.pendingViewToScroll = view;
          return;
        }
        scrollMaxY = scroller.scrollHeight - scroller.clientHeight;
      }
      
      if (this.pendingViewToScroll) {
        view = this.pendingViewToScroll;
        this.pendingViewToScroll = null;
        console.warn("Using pending view to scroll")
      }


      const viewMaxY = this.scene.bounds.h - this.window.height + this.marginTop;
      const panY = (view.y + this.marginTop + view.h/2) - this.window.height/2;
      const scrollRatio = panY / viewMaxY;
      const scrollTop = scrollRatio * scrollMaxY;
      

      if (this.scrollbar) {
        this.scrollbar.scroll([0, (scrollRatio * 100) + "%"]);
      } else if (this.fullpage) {
        window.scrollTo(window.scrollX, scrollTop);
      } else {
        scroller.scroll({
          top: scrollTop,
        })
      }

      return scrollRatio;

    },

    pushScrollToView(scrollRatio, transition) {

      if (this.ignoreScrollToViewUntil != null && Date.now() < this.ignoreScrollToViewUntil) {
        return;
      }
      this.ignoreScrollToViewUntil = null;

      if (!this.nativeScroll) {
        console.warn("Pushing scroll to view while not in native scrolling mode");
      }

      if (!this.$refs.scroller) return;

      const viewMaxY = (this.scene?.bounds?.h || 0) - this.window.height + this.marginTop;
      
      let scrollY = 0;
      if (scrollRatio == null) {
        if (this.scrollbar) {
          const scroll = this.scrollbar.scroll();
          // Uncomment for scroll position debugging
          // console.log(scroll.position, scroll.max, scroll.ratio, scroll.position.y / viewMaxY)
          scrollRatio = scroll.ratio.y;
          scrollY = scroll.position.y;
        } else {
          const scroller = this.$refs.scroller;
          const scrollMaxY = 
            this.fullpage ?
              document.body.scrollHeight - window.innerHeight :
              scroller.scrollHeight - scroller.clientHeight;
          const scrollTop =
            this.fullpage ?
              window.scrollY :
              scroller.scrollTop;
          scrollRatio = scrollMaxY ? scrollTop / scrollMaxY : 0;
          scrollY = scrollTop;
        }
      }

      // Ratio can be outside of range if the range has changed recently
      scrollRatio = Math.min(1, Math.max(0, scrollRatio));

      const viewY = scrollRatio * viewMaxY;
      const view = {
        x: 0,
        y: viewY - this.marginTop,
        w: this.window.width,
        h: this.window.height,
      }
      this.$refs.viewer.setView(view, transition && { animationTime: transition });
      
      // Offset the native browser scroll to keep the viewer visible
      this.$refs.viewer.$el.style.transform = `translate(0, ${scrollY}px)`;

      this.visibleRegionsTask.perform(view, this.sceneParams);
    },

  }
};
</script>

<style scoped>

.spinner {
  position: fixed;
  --size: 200px;
  top: calc(50% - var(--size)/2);
  left: calc(50% - var(--size)/2);
  width: var(--size);
  height: var(--size);
}
.container {
  position: relative;
}

.container .progress {
  position: fixed;
}

.container .scroller {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  overflow-y: auto;
}

.container.fullpage .scroller {
  /* overflow-x: hidden; */
  overflow-y: visible;
  /* width: 100vw; */
}

.container .scroller.disabled {
  pointer-events: none;
  /* overflow-y: hidden; */
}

.container .viewer {
  position: absolute;
  top: 0;
  left: 0;
}

.container.fullpage .viewer {
  position: absolute;
  width: 100vw;
  height: 100vh;
  /* Fix for mobile browsers */
  height: calc(var(--vh, 1vh) * 100);
  margin-top: -64px;
}

.container.fullpage.fixed .viewer {
  position: fixed;
  margin-top: 0;
  transform: translate(0, 0) !important;
}

.context-menu {
  position: fixed;
  width: fit-content;
}

</style>
