<template>
  <div class="container">

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
      :view="view"
      @zoom="onZoom"
      @pan="onPan"
      @view="onView"
      @click="onClick"
      @load="onLoad"
      @keydown="onKeyDown"
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
    >
      <div
        class="virtual-canvas"
        ref="virtualCanvas"
        :style="{ height: canvas.height + 'px' }">
      </div>
    </div>
  </div>
</template>

<script>
import { computed, nextTick, ref, toRef, watch, watchEffect } from 'vue';
import { debounce, throttle, waitDebounce } from '../utils';
import TileViewer from './TileViewer.vue';
import { getCollection, getRegion, getRegions, getScene, useCollectionTask, useRegionTask, useSceneTask } from '../api';
import { timeout, useTask, useTaskGroup } from "vue-concurrency";
import PageTitle from './PageTitle.vue';

export default {

  props: [
    "collectionId",
    "regionId",
    "options",
  ],

  emits: {
    load: null,
    scene: null,
    tasks: null,
  },

  components: {
    TileViewer,
    PageTitle,
  },

  data() {
    return {
      loadProgress: 0,
      cacheKey: "",
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

    const tasks = useTaskGroup({ collectionTask, sceneTask, regionTask })
    
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
    this.$refs.scroller.addEventListener("scroll", this.onScroll);
    this.$emit("tasks", this.tasks);
  },
  unmounted() {
    clearInterval(this.demoInterval);
    this.removeResizeObserver();
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
        imageHeight: this.options.image.height,
        sceneWidth: this.window.width,
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
    sceneParams(sceneParams) {
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
      if (this.region && this.regionFocusPending) {
        this.view = this.region.bounds;
        this.onView(this.view);
        this.regionFocusPending = null;
      }
    },
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
    
    async simulate() {
      this.simulationStart = Date.now();
      this.simulationPrewait = 3000;
      this.simulationPostwait = 3000;
      this.simulationDuration = 10000;
      const promise = new Promise(resolve => {
        this.simulateNext(resolve);
      });
      await promise;
    },

    simulateNext(resolve) {
      const now = Date.now();
      const elapsed = now - this.simulationStart;
      const ratio = Math.max(0, Math.min(1, (elapsed - this.simulationPrewait) / this.simulationDuration));
      if (elapsed >= this.simulationDuration + this.simulationPrewait + this.simulationPostwait) {
        resolve();
        return;
      }
      const height = this.viewer.scene.height - this.window.height;
      const y = ratio * height;
      this.$refs.scroller.scroll(0, y);
      window.requestAnimationFrame(this.simulateNext.bind(this, resolve));
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
      this.lastPointerDownEvent = event;
    },

    async onPointerUp(event) {
      if (!this.nativeScroll) {
        return;
      }
      this.lastPointerUpEvent = event;
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
        this.redispatchEventToViewer(down);
        this.redispatchEventToViewer(up);
      }
    },

    async onTouchStart(event) {
      if (!this.nativeScroll) {
        return;
      }
      console.log(event);
      if (this.nativeScroll && event.touches.length >= 2) {
        this.nativeScroll = false;
        await Vue.nextTick();
        this.redispatchEventToViewer(this.lastTouchStartEvent);
        this.redispatchEventToViewer(event);
     }
      this.lastTouchStartEvent = event;
    },

    async onTouchEnd(event) {
      if (!this.nativeScroll) {
        return;
      }
      this.lastTouchStartEvent = null;
      console.log(event);
    },

    async onZoom(zoom) {
      if (zoom < 0.99) {
        this.nativeScroll = true;
        this.pushScrollToView();
      }
    },

    async onPan(event) {
      if (this.nativeScroll) return;
      this.region = null;
      this.pushViewToScroll(event);
    },

    async onView(view) {
      this.pushViewToScroll(view);
    },

    async onClick(event) {
      const regions = await getRegions(event.x, event.y, 0, 0, this.sceneParams);
      if (regions && regions.length > 0) {
        const region = regions[0];
        console.log(region);
        this.focusRegion(region, 2);
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
      this.$refs.viewer.setView(
        region.bounds,
        transition && { animationTime: transition }
      )
      this.onView(region.bounds);
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

    async navigateExit() {
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

      const scroller = this.$refs.scroller;
      
      if (!scroller || this.viewer.scene.height == 0) {
        this.pendingViewToScroll = view;
        return;
      }
      if (this.pendingViewToScroll) {
        view = this.pendingViewToScroll;
        this.pendingViewToScroll = null;
      }

      const viewMaxY = this.viewer.scene.height - this.window.height;
      const scrollMaxY = scroller.scrollHeight - scroller.clientHeight;
      const panY = (view.y + view.h/2) - this.window.height/2;
      const scrollRatio = panY / viewMaxY;
      const scrollTop = scrollRatio * scrollMaxY;

      scroller.scroll({
        top: scrollTop,
      })
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

      const scrollMaxY = scroller.scrollHeight - scroller.clientHeight;
      const scrollRatio = scrollMaxY ? scroller.scrollTop / scrollMaxY : 0;
      const viewY = scrollRatio * viewMaxY;

      const options = {}

      const view = {
        x: 0,
        y: viewY,
        w: this.window.width,
        h: this.window.height,
      }

      if (transition !== undefined) {
        this.$refs.viewer.setView(view, { animationTime: transition });
      } else {
        this.view = view;
      }

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
  overflow-y: auto;
  width: 100%;
  height: 100%;
}

.container .scroller.disabled {
  pointer-events: none;
  overflow-y: hidden;
}

.container .viewer {
  position: absolute;
  top: 0;
  left: 0;
}

</style>
