<template>
  <div class="container">
    <tile-viewer
      class="viewer"
      ref="viewer"
      :interactive="!nativeScroll"
      :api="api"
      :scene="viewer.scene"
      :view="view"
      @zoom-out="onZoomOut"
      @pan="onPan"
      @load="onLoad"
    ></tile-viewer>
    <div
      class="scroller"
      :class="{ disabled: !nativeScroll }"
      ref="scroller"
      @pointerDown="onPointerDown"
      @pointerUp="onPointerUp"
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
import { nextTick } from 'vue';
import { debounce, LatestFetcher, throttle, waitDebounce } from '../utils';
import TileViewer from './TileViewer.vue';

export default {

  props: [
    "api",
    "collection",
  ],

  emits: {
    load: null,
    scene: null,
  },

  components: {
    TileViewer,
  },

  data() {
    return {
      loadProgress: 0,
      imageHeight: 30,
      cacheKey: "",
      scene: {
        bounds: {
          w: 0,
          h: 0
        }
      },
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
      viewer: {
        scene: {
          width: 0,
          height: 0,
          params: null,
        }
      },
      nativeScroll: true,
    }
  },
  async mounted() {
    this.addResizeObserver();
    this.$refs.scroller.addEventListener("scroll", this.onScroll);

    this.fetchScene = LatestFetcher();
    await this.updateScene();
    this.updateScrollToView();

    // this.demoScroll();
  },
  unmounted() {
    clearInterval(this.demoInterval);
    this.removeResizeObserver();
  },
  computed: {
    sceneParams() {
      const params = {
        collection: this.collection ? this.collection.id : null,
        imageHeight: this.imageHeight,
        sceneWidth: this.window.width,
        cacheKey: this.cacheKey,
      };
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
    },
  },
  watch: {
    sceneParams(sceneParams) {
      this.debouncedUpdateScene();
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
      this.updateScrollToView();
    },

    onScroll() {
      if (!this.nativeScroll) return;
      this.updateScrollToView();
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

    async onZoomOut(event) {
      this.nativeScroll = true;
      this.updateScrollToView();
    },

    async onPan(event) {
      if (this.nativeScroll) return;
      const scroller = this.$refs.scroller;
      const viewMaxY = this.viewer.scene.height - this.window.height;
      const scrollMaxY = scroller.scrollHeight - scroller.clientHeight;
      const panY = event.y - this.window.height * 0.5;
      const scrollRatio = panY / viewMaxY;
      const scrollTop = scrollRatio * scrollMaxY;
      scroller.scroll({
        top: scrollTop,
      })
    },

    async onClick(event) {
      if (!this.nativeScroll) {
        return;
      }

      this.nativeScroll = false;
      await nextTick();

      this.redispatchEventToViewer(this.lastPointerDownEvent);
      this.redispatchEventToViewer(this.lastPointerUpEvent);

    },

    updateScrollToView() {

      if (!this.$refs.scroller) return;
      
      const scroller = this.$refs.scroller;

      const viewMaxY = this.viewer.scene.height - this.window.height;

      const scrollMaxY = scroller.scrollHeight - scroller.clientHeight;
      const scrollRatio = scrollMaxY ? scroller.scrollTop / scrollMaxY : 0;
      const viewY = scrollRatio * viewMaxY;

      this.view = {
        x: 0,
        y: viewY,
        width: this.window.width,
        height: this.window.height,
      }
    },

    
    debouncedUpdateScene: waitDebounce(function() {
      this.updateScene();
    }, 200),

    async updateScene() {
      if (this.collection == null) {
        return;
      }
      if (!this.sceneParams) {
        return;
      }
      if (this.sceneParams == this.sceneParamsLoaded) {
        return;
      }
      this.sceneParamsLoaded = this.sceneParams;
      const url = `${this.api}/scenes?${this.sceneParams}`;
      try {
        this.$emit("load", { scene: true });
        const scenes = await this.fetchScene(url);
        this.$emit("load", { scene: false });
        if (!scenes || scenes.length < 1) {
          throw new Error("Scene not found");
        }
        this.scene = scenes[0];
        this.$emit("scene", this.scene);
        this.viewer.scene = {
          width: this.scene.bounds.w,
          height: this.scene.bounds.h,
          params: this.sceneParams,
        }
      } catch (error) {
        if (error.name == "AbortError") return;
        throw error;
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
}

.container .viewer {
  position: absolute;
  top: 0;
  left: 0;
}

</style>
