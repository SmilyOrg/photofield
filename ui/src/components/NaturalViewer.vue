<template>
  <div class="container" :class="{ fullpage }">

    <page-title :title="pageTitle"></page-title>
    
    <!-- <div>
      <div v-if="collectionTask.isRunning">Loading...</div>
      <div v-else-if="collectionTask.isError">
        <p>{{ collectionTask.last.error.message }}</p>
        <button @click="collectionTask.perform">Try again</button>
      </div>
      <h2>Collection</h2><pre>{{ collection }}</pre>
      <h2>Scene Params</h2><pre>{{ sceneParams }}</pre>
      <h2>Scene</h2><pre>{{ scene }}</pre>
      <h2>Region</h2><pre>{{ region }}</pre>
    </div> -->
    
    <tile-viewer
      class="viewer"
      ref="viewer"
      :interactive="!nativeScroll"
      :scene="viewer.scene"
      :tileSize="tileSize"
      @zoom="onZoom"
      @view="onView"
      @click="onClick"
      @load="onLoad"
      @keydown="onKeyDown"
      @contextmenu.prevent="onContext"
    ></tile-viewer>
    <div
      class="scroller"
      :class="{ disabled: !nativeScroll }"
      ref="scroller"
      @pointerDown="onPointerDown"
      @pointerUp="onPointerUp"
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
        :scene="viewer.scene"
        :sceneParams="sceneParams"
        :flipX="contextFlipX"
        :flipY="contextFlipY"
        @close="$refs.contextMenu.close()"
      ></region-menu>
    </ContextMenu>
  </div>
</template>

<script>
import { computed, nextTick, ref, toRef, watch, watchEffect } from 'vue';
import { debounce, throttle, waitDebounce } from '../utils';
import TileViewer from './TileViewer.vue';
import { getCollection, getRegion, getRegions, getScene, useCollectionTask, useRegionTask, useSceneTask } from '../api';
import { timeout, useTask, useTaskGroup } from "vue-concurrency";
import PageTitle from './PageTitle.vue';
import Simulation from '../simulation';
import ContextMenu from '@overcoder/vue-context-menu';
import RegionMenu from './RegionMenu.vue';

export default {

  props: [
    "collectionId",
    "regionId",
    "cacheKey",
    "settings",
    "fullpage",
  ],

  emits: {
    load: null,
    tasks: null,
    immersive: Boolean,
  },

  components: {
    TileViewer,
    PageTitle,
    ContextMenu,
    RegionMenu,
  },

  data() {
    return {
      tileSize: 512,
      loadProgress: 0,
      window: {
        x: 0,
        y: 0,
        width: 0,
        height: 0,
      },
      view: {
        x: 0,
        y: 0,
        width: 0,
        height: 0,
      },
      nativeScroll: true,
      contextRegion: null,
      contextAnchor: "",
      contextFlipX: false,
      contextFlipY: false,
    }
  },

  setup(props) {
    const collectionId = toRef(props, "collectionId");

    const collectionTask = useCollectionTask();
    const collection = computed(() => collectionTask.last?.value);
    watch(collectionId, id => {
      collectionTask.perform(id);
    })
    collectionTask.perform(collectionId.value);

    const sceneTask = useSceneTask().restartable();
    const scene = computed(() => sceneTask.last?.value);

    const regionTask = useRegionTask().restartable();
    const region = computed(() => {
      if (regionTask.isRunning) return undefined;
      return regionTask.last.value;
    });

    const regionSeekId = ref(null);
    const regionSeekApplyTask = useTask(function*(_, router) {
      yield timeout(1000);

      const seekId = regionSeekId.value;
      regionSeekId.value = null;
      router.push({
        name: "region",
        params: {
          collectionId: collectionId.value,
          regionId: seekId,
        },
      });
    }).restartable();

    const tasks = useTaskGroup({
      collectionTask,
      sceneTask,
      regionTask,
    })
    
    return {
      collectionTask,
      collection,
      sceneTask,
      scene,
      regionTask,
      region,
      regionSeekId,
      regionSeekApplyTask,
      tasks,
    }
  },

  async mounted() {
    if (this.regionId != null) {
      this.regionFocusPending = true;
      this.nativeScroll = false;
    }
    this.addResizeObserver();
    // this.$refs.scroller.addEventListener("scroll", this.onScroll);
    this.$emit("tasks", this.tasks);
    this.$bus.on("home", this.navigateExit);
    this.$bus.on("simulate-run", this.simulate);
    if (this.fullpage) this.addFullpageListeners();
    // this.simulate();
  },
  unmounted() {
    clearInterval(this.demoInterval);
    this.removeResizeObserver();
    this.$bus.off("home", this.navigateExit);
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
        cacheKey: this.cacheKey,
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
      const aspectRatio = this.viewer.scene.width / this.viewer.scene.height;
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
    }
  },
  watch: {
    fullpage(fullpage) {
      if (fullpage) {
        addFullpageListeners();
      } else {
        removeFullpageListeners();
      }
    },
    async sceneParams(sceneParams) {
      this.sceneTask.perform(sceneParams);
      this.regionTask.perform(this.regionId, sceneParams);
    },
    regionId: {
      immediate: true,
      async handler(regionId) {
        // console.log("regionId", regionId)
        const recentlyFocused = this.focusRegionTime !== undefined && Date.now() - this.focusRegionTime < 200;
        if (this.regionId != null && !recentlyFocused) {
          this.nativeScroll = false;
          this.regionFocusPending = true;
        }
        this.regionTask.perform(regionId, this.sceneParams);
      },
    },
    region(region) {
      // console.log("region", this.regionFocusPending, this.region)
      if (this.region && this.regionFocusPending) {
        this.viewRegion(this.region);
        this.regionFocusPending = null;
      }
    },
    nativeScroll(nativeScroll) {
      this.$emit("immersive", !nativeScroll);
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
    },

    removeFullpageListeners() {
      window.removeEventListener('scroll', this.onScroll);
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

      this.simulation = new Simulation(tileEvaluation);
      const results = await this.simulation.run(this);
      console.log(JSON.stringify(results, null, 2));
      this.$bus.emit("simulate-done");
    },

    demoScroll() {
      const y = (1 + Math.sin(Date.now() * Math.PI * 2 / 1000 * 0.05)) / 2 * (this.viewer.scene.height - this.window.height);
      this.$refs.scroller.scroll(0, y);
      window.requestAnimationFrame(this.demoScroll);
    },

    onResize(rect) {
      if (rect.width == 0 || rect.height == 0) return;
      this.window = rect;
      if (this.nativeScroll) {
        this.pushScrollToView();
      }
    },

    onScroll(event) {
      if (!this.nativeScroll) return;
      this.pushScrollToView();
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
      // console.log("UP", event);
      const down = this.lastPointerDownEvent;
      const up = this.lastPointerUpEvent;
      const duration = up.timeStamp - down.timeStamp;
      const dx = up.screenX - down.screenX;
      const dy = up.screenY - down.screenY;
      const distance = Math.sqrt(dx*dx + dy*dy);
      const quick = duration < this.pointerTimeThreshold && distance < this.pointerDistThreshold;
      if (quick) {
        this.nativeScroll = false;
        await nextTick();
        const pos = this.$refs.viewer.elementToViewportCoordinates(down);
        this.onClick(pos);
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
      if (zoom < 0.99) {
        this.nativeScroll = true;
        this.pushScrollToView();
      }
    },

    // async onPan(view) {
    //   if (this.nativeScroll) return;
    //   this.region = null;
    //   this.view = view;
    //   this.pushViewToScroll(view);
    // },

    async onView(view) {
      this.view = view;
      if (this.nativeScroll) return;
      this.pushViewToScroll(view);
    },

    async onClick(event) {
      const regions = await getRegions(event.x, event.y, 0, 0, this.sceneParams);
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
      }
    },

    async onContext(event) {
      this.contextRegion = null;
      this.$refs.contextMenu.open(event);
      const menuWidth = 250;
      const menuHeight = 300;
      const right = event.x + menuWidth;
      const bottom = event.y + menuHeight;
      this.contextFlipX = right > window.innerWidth;
      this.contextFlipY = bottom > window.innerHeight;
      const pos = this.$refs.viewer.elementToViewportCoordinates(event);
      const regions = await getRegions(pos.x, pos.y, 0, 0, this.sceneParams);
      if (regions && regions.length > 0) {
        this.contextRegion = regions[0];
      }
    },

    async focusRegion(region, transition) {
      this.viewRegion(region, transition);
      this.focusRegionTime = Date.now();
      this.$router.push({
        name: "region",
        params: {
          collectionId: this.collection.id,
          regionId: region?.id,
        },
      })
    },

    async viewRegion(region, transition) {
      this.setView(region.bounds, transition);
      this.onView(region.bounds);
    },

    setView(view, transition) {
      // console.log(view, transition);
      this.$refs.viewer.setView(
        view,
        transition && { animationTime: transition }
      )
      this.view = view;
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
      if (nextId < 0 || nextId >= this.scene.photoCount-1) {
        return;
      }
      this.regionFocusPending = true;
      this.regionSeekId = nextId;
      this.regionTask.perform(nextId, this.sceneParams);
      this.regionSeekApplyTask.perform(this.$router);
    },

    navigateExit() {
      this.nativeScroll = true;
      this.pushScrollToView(1);
      // Firefox fires an onScroll event when you make the scrollbar
      // visible again, which breaks the smooth transition. This ignores scroll
      // events until a short time after exiting navigation to prevent this.
      this.ignoreScrollToViewUntil = Date.now() + 200;
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

      if (this.viewer.scene.height == 0) {
        console.warn("Scene has zero height, view to scroll pending", view);
        this.pendingViewToScroll = view;
        return;
      }

      const scroller = this.$refs.scroller;
      let scrollMaxY;
      if (this.fullpage) {
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
      }

      const viewMaxY = this.viewer.scene.height - this.window.height;
      const panY = (view.y + view.h/2) - this.window.height/2;
      const scrollRatio = panY / viewMaxY;
      const scrollTop = scrollRatio * scrollMaxY;

      if (this.fullpage) {
        window.scrollTo(window.scrollX, scrollTop);
      } else {
        scroller.scroll({
          top: scrollTop,
        })
      }
    },

    pushScrollToView(transition) {

      if (this.ignoreScrollToViewUntil != null && Date.now() < this.ignoreScrollToViewUntil) {
        return;
      }
      this.ignoreScrollToViewUntil = null;

      if (!this.nativeScroll) {
        console.warn("Pushing scroll to view while not in native scrolling mode");
      }

      if (!this.$refs.scroller) return;
      
      const scroller = this.$refs.scroller;

      const viewMaxY = this.viewer.scene.height - this.window.height;

      const scrollMaxY = 
        this.fullpage ?
          document.body.scrollHeight - window.innerHeight :
          scroller.scrollHeight - scroller.clientHeight;

      const scrollTop =
        this.fullpage ?
          window.scrollY :
          scroller.scrollTop;
      
      const scrollRatio = scrollMaxY ? scrollTop / scrollMaxY : 0;
      const viewY = scrollRatio * viewMaxY;

      const view = {
        x: 0,
        y: viewY,
        w: this.window.width,
        h: this.window.height,
      }

      this.$refs.viewer.setView(view, transition && { animationTime: transition });

    },

  }
};
</script>

<style scoped>

.container {
  position: relative;
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
  position: fixed;
  width: 100vw;
}

.context-menu {
  position: absolute;
  margin-top: -60px;
  width: fit-content;
}

</style>
