<template>
  <div class="app">
    <ui-top-app-bar
      class="top-bar"
      nav-id="menu"
      :fixed="true"
      contentSelector="#content"
      @nav="drawer = !drawer"
    >
      {{ tasks?.collectionTask.last?.value?.name || "Photos" }}

      <template #toolbar="{ toolbarItemClass }">
        <div class="size-icons">
          <ui-icon-button
            icon="photo_size_select_small"
            :class="{ active: options.image.height == 30, toolbarItemClass }"
            @click="options.image.height = 30"
            outlined
          >
          </ui-icon-button>
          <ui-icon-button
            icon="photo_size_select_large"
            :class="{ active: options.image.height == 100, toolbarItemClass }"
            @click="options.image.height = 100"
            outlined
          >
          </ui-icon-button>
          <ui-icon-button
            icon="photo_size_select_actual"
            :class="{ active: options.image.height == 300, toolbarItemClass }"
            @click="options.image.height = 300"
            outlined
          >
          </ui-icon-button>
        </div>
        <ui-spinner
          class="small-spinner"
          size="small"
          active
          :closed="load.image.inProgress == 0"
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
        :options="options"
        :class="{ simulating }"
        @load="onLoad"
        @scene="onScene"
        @tasks="onTasks"
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
      options: {
        image: {
          height: 100,
        },
      },
      load: {
        image: 0,
      },
      tasks: null,
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
    loading() {
      if (!this.tasks) return false;
      return (
        this.tasks.sceneTask.isRunning ||
        this.tasks.collectionTask.isRunning
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
    },
    onScene(scene) {
      this.scene = scene;
    },
    onTasks(tasks) {
      this.tasks = tasks;
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

.size-icons {
  margin: 10px;
}

.size-icons button:before {
  border-radius: 0;
  opacity: 0.03;
}

.size-icons button.active::before {
  opacity: var(--mdc-ripple-focus-opacity,0.24);
}

.size-icons button:first-child::before {
  border-top-left-radius: 5px;
  border-bottom-left-radius: 5px;
}

.size-icons button:last-child::before {
  border-top-right-radius: 5px;
  border-bottom-right-radius: 5px;
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