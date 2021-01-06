<template>
  <div class="app">
    <ui-top-app-bar
      class="top-bar"
      nav-id="menu"
      :fixed="true"
      contentSelector="#content"
      @nav="drawer = !drawer"
    >
      <span class="title" @click="onTitleClick()">
        {{ tasks?.collectionTask.last?.value?.name || "Photos" }}
      </span>
      <span
        class="files"
        v-if="fileCount != null"
      >
        {{ fileCount }} files
      </span>

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
          {{ collections.length }} collections
        </ui-drawer-subtitle>
      </ui-drawer-header>
      <ui-divider></ui-divider>
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
          <ui-item>
            <ui-button @click="refreshCache()">
              Refresh Cache
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
        :cacheKey="cacheKey"
        :class="{ simulating }"
        @load="onLoad"
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
      cacheKey: "",
      load: {
        image: 0,
      },
      tasks: null,
      drawer: false,
      simulating: false,
      collections: [],
    }
  },
  async mounted() {
    this.cacheKey = localStorage.cacheKey || "";
    this.collections = await getCollections();
    if (!this.$route.params.collectionId) {
      this.drawer = true;
    }
  },
  computed: {
    scene() {
      return this.tasks?.sceneTask.last?.value;
    },
    loading() {
      if (!this.tasks) return false;
      return (
        this.tasks.sceneTask.isRunning ||
        this.tasks.collectionTask.isRunning
      );
    },
    fileCount() {
      return this.scene?.photoCount !== undefined ?
        this.scene.photoCount.toLocaleString() : 
        null;
    }
  },
  methods: {
    refreshCache() {
      this.cacheKey = Date.now();
      localStorage.cacheKey = this.cacheKey;
    },
    onTitleClick() {
      this.$bus.emit("home");
    },
    onLoad(load) {
      this.load = { ...this.load, ...load };
    },
    onTasks(tasks) {
      this.tasks = tasks;
    },
    async simulate() {
      this.drawer = false;
      this.simulating = true;
      const done = () => {
        this.simulating = false;
        this.$bus.off("simulate-done", done);
      }
      this.$bus.on("simulate-done", done);
      this.$bus.emit("simulate");
    }
  }
}
</script>

<style scoped>

.top-bar {
  --mdc-theme-primary: white;
  --mdc-theme-on-primary: rgba(0,0,0,.87); 
}

.title {
  cursor: pointer;
}

.files {
  font-size: 0.8em;
  margin-left: 12px;
  color: var(--mdc-theme-text-hint-on-background);
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