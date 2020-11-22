<template>
  <div class="app">
    <ui-top-app-bar
      class="top-bar"
      :fixed="true"
      contentSelector="#content"
    >
      Photos
      <!-- <template #toolbar>
      </template> -->
    </ui-top-app-bar>
    <div id="content">
      <div class="loading-overlay" :class="{ active: loading }">
        <ui-spinner
          :active="loading"
          class="spinner"
          size="large"
        ></ui-spinner>
      </div>
      <natural-viewer
        class="viewer"
        :api="api"
        @load="onLoad"
      ></natural-viewer>
    </div>
  </div>
</template>

<script>
import NaturalViewer from './components/NaturalViewer.vue'

export default {
  name: 'App',
  components: {
    NaturalViewer,
  },
  data() {
    return {
      api: "https://photos.pelun.net",
      load: {
        scene: false,
      },
      loading: false,
    }
  },
  methods: {
    onLoad(load) {
      this.load = { ...this.load, ...load };
      this.loading = this.load.scene;
    }
  }
}
</script>

<style scoped>

.top-bar {
  --mdc-theme-primary: white;
  --mdc-theme-on-primary: rgba(0,0,0,.87); 
}

.spinner {
  --mdc-theme-primary: white;
}

.loading-overlay {
  display: none;
  position: absolute;
  z-index: 10;
  background: rgba(0, 0, 0, 0.3);
  width: 100%;
  height: calc(100vh - 64px);
  align-items: center;
  justify-content: center;
}

.loading-overlay.active {
  display: flex;
}

.viewer {
  /* width: 400px;
  height: 400px; */
  height: calc(100vh - 64px);
  /* width: 100%;
  height: 100%; */
}
</style>