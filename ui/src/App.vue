<template>
  <div class="app">
    <ui-top-app-bar
      class="top-bar"
      nav-id="menu"
      :fixed="true"
      contentSelector="#content"
      @nav="drawer = !drawer"
    >
      {{ collection ? collection.name : "Photos" }}
      <template #toolbar="{ toolbarItemClass }">
        <ui-spinner
          active
          :closed="load.image.inProgress == 0"
          size="small"
          class="small-spinner"
          :class="toolbarItemClass"
        ></ui-spinner>
      </template>
    </ui-top-app-bar>
    <ui-drawer type="modal" nav-id="menu" v-model="drawer">
      <ui-drawer-header>
        <ui-drawer-title>Photos</ui-drawer-title>
        <ui-drawer-subtitle>
          {{ fileCount }} files
        </ui-drawer-subtitle>
      </ui-drawer-header>
      <ui-drawer-content>
        <ui-nav>
          <ui-nav-item
            v-for="collection in collections"
            :key="collection.id"
            :href="'/collections/' + collection.id"
          >
            {{ collection.name }}
          </ui-nav-item>
          <ui-item>
            <ui-button @click="simulate()">
              Simulate
            </ui-button>
          </ui-item>
        </ui-nav>
      </ui-drawer-content>
    </ui-drawer>
    <ui-drawer-backdrop></ui-drawer-backdrop>
    <div id="content">
      <div class="loading-overlay" :class="{ active: loading }">
        <ui-spinner
          :active="loading"
          class="spinner"
          size="large"
        ></ui-spinner>
      </div>
      <router-view
        class="viewer"
        ref="viewer"
        :class="{ simulating }"
        @load="onLoad"
        @scene="onScene"
      >
      </router-view>
    </div>
  </div>
</template>

<script>
import { getCollections } from './api';
import NaturalViewer from './components/NaturalViewer.vue'

export default {
  name: 'App',
  components: {
    NaturalViewer,
  },
  data() {
    return {
      load: {
        scene: false,
        image: 0,
      },
      loading: false,
      drawer: false,
      simulating: false,
      collections: [],
      scene: {},
    }
  },
  async mounted() {
    this.collections = await getCollections();
  },
  computed: {
    collection() {
      const id = this.$route.params.collectionId;
      if (!this.collections) return null;
      return this.collections.find(
        collection => collection.id == id
      );
    },
    fileCount() {
      return this.scene.photoCount !== undefined ?
        this.scene.photoCount.toLocaleString() : 
        "No";
    }
  },
  methods: {
    onLoad(load) {
      this.load = { ...this.load, ...load };
      this.loading = this.load.scene;
    },
    onScene(scene) {
      this.scene = scene;
    },
    async simulate() {
      this.drawer = false;
      this.simulating = true;
      await this.$refs.viewer.simulate();
      this.simulating = false;
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

.small-spinner {
  --mdc-theme-primary: var(--mdc-theme-on-primary);
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

.viewer.simulating {
  width: 1280px;
  height: 720px;
}

</style>