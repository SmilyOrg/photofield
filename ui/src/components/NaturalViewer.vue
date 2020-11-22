<template>
  <div class="container">
    <tile-viewer
      class="viewer"
      ref="viewer"
      :interactive="!nativeScroll"
      :api="api"
      :scene="viewer.scene"
      :view="view"
    ></tile-viewer>
    <div class="scroller" ref="scroller">
      <div
        class="virtual-canvas"
        ref="virtualCanvas"
        :style="{ height: canvas.height + 'px' }">
      </div>
    </div>
  </div>
</template>

<script>
import { debounce, LatestFetcher, throttle, waitDebounce } from '../utils';
import TileViewer from './TileViewer.vue';

export default {

  props: [
    "api",
  ],

  emits: {
    load: null,
  },

  components: {
    TileViewer,
  },

  data() {
    return {
      collection: {
        // id: "2020-10-ruhrgebiet",
        id: "2018-08-usa",
      },
      imageHeight: 30,
      // imageHeight: 100,
      // imageHeight: 300,
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
    this.fetchScene = LatestFetcher();
    await this.updateScene();
    this.resizeObserver = new ResizeObserver(entries => {
      this.onResize(entries[0].contentRect);
    });
    this.resizeObserver.observe(this.$refs.viewer.$el);

    this.$refs.scroller.addEventListener("scroll", this.onScroll);

    // this.demoScroll();
  },
  unmounted() {
    clearInterval(this.demoInterval);
    if (this.resizeObserver) {
      this.resizeObserver.disconnect();
      this.resizeObserver = null;
    }
    // if (this.$refs.scroller.removeEventListener("scroll", this.onScroll);
  },
  computed: {
    sceneParams() {
      const params = {
        collection: this.collection ? this.collection.id : null,
        imageHeight: this.imageHeight,
        sceneWidth: this.window.width,
        cacheKey: this.cacheKey,
      };
      return Object.entries(params).map(([key, value]) => `${key}=${value}`).join("&");
    },
    canvas() {
      const aspectRatio = this.viewer.scene.width / this.viewer.scene.height;
      return {
        width: this.window.width,
        height: this.window.width / aspectRatio,
      }
    },
  },
  watch: {
    sceneParams(sceneParams) {
      this.debouncedUpdateScene();
    },
  },
  methods: {

    demoScroll() {
      const y = (1 + Math.sin(Date.now() * Math.PI * 2 / 1000 * 0.05)) / 2 * (this.viewer.scene.height - this.window.height);
      this.$refs.scroller.scroll(0, y);
      window.requestAnimationFrame(this.demoScroll);
    },

    onResize(rect) {
      if (rect.width == 0 || rect.height == 0) return;
      this.window = rect;
    },

    onScroll() {
      this.updateScrollToView();
    },

    updateScrollToView() {

      if (!this.$refs.scroller) return;
      
      const scroller = this.$refs.scroller;

      const viewMaxY = this.viewer.scene.height - this.window.height;

      const scrollMaxY = scroller.scrollHeight - scroller.clientHeight;
      const scrollRatio = scrollMaxY ? scroller.scrollTop / scrollMaxY : 0;
      const viewY = scrollRatio * viewMaxY;

      this.view = {
        x: this.view.x,
        y: viewY,
        width: this.window.width,
        height: this.window.height,
      }

      // console.log("onScroll", scrollRatio, viewMaxY, viewY);
    },

    
    debouncedUpdateScene: waitDebounce(function() {
      this.updateScene();
    }, 200),

    async updateScene() {
      if (this.collection == null) {
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
  overflow-y: scroll;
  width: 100%;
  height: 100%;
}

.container .viewer {
  position: absolute;
  top: 0;
  left: 0;
}

</style>
