<template>
  <div class="app">
    <ui-top-app-bar
      class="top-bar"
      :class="{ immersive }"
      nav-id="menu"
      :fixed="true"
      contentSelector="#content"
      @nav="drawer = !drawer"
    >
      <span class="title" @click="onTitleClick()">
        {{ collection?.name || "Photos" }}
      </span>
      
      <span class="collection-menu">
        <ui-button
          class="files"
          v-if="fileCount != null"
          @click="collectionMenuOpen = !collectionMenuOpen"
        >
          {{ indexTasks?.items[0]?.count || fileCount }} files
          <template #after>
            <ui-icon>expand_more</ui-icon>
          </template>
        </ui-button>
          <ui-menu
            v-model="collectionMenuOpen"
          >
            <ui-menuitem @click="reindex()">Reindex Collection</ui-menuitem>
          </ui-menu>
      </span>

      <template #toolbar="{ toolbarItemClass }">
        <div class="settings" :class="{ hidden: !settingsExpanded, toolbarItemClass }">
          <ui-select
            v-model="settings.layout"
            :options="layoutOptions"
          >
            Layout
          </ui-select>
          <div class="size-icons">
            <ui-icon-button
              icon="photo_size_select_small"
              :class="{ active: settings.image.height == 30 }"
              @click="settings.image.height = 30"
              outlined
            >
            </ui-icon-button>
            <ui-icon-button
              icon="photo_size_select_large"
              :class="{ active: settings.image.height == 100 }"
              @click="settings.image.height = 100"
              outlined
            >
            </ui-icon-button>
            <ui-icon-button
              icon="photo_size_select_actual"
              :class="{ active: settings.image.height == 300 }"
              @click="settings.image.height = 300"
              outlined
            >
            </ui-icon-button>
          </div>
        </div>
        <ui-icon-button
          icon="settings"
          class="settings-toggle"
          :class="{ expanded: settingsExpanded, toolbarItemClass }"
          @click="settingsExpanded = !settingsExpanded"
        >
        </ui-icon-button>
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
        :settings="settings"
        :cacheKey="cacheKey"
        :class="{ simulating }"
        :fullpage="true"
        @load="onLoad"
        @tasks="onTasks"
        @immersive="onImmersive"
      >
      </router-view>
    </div>
  </div>
</template>

<script>
import { getCollections, reindexCollection, useIndexTasks } from './api';
import { useRouter, useRoute } from 'vue-router'
import NaturalViewer from './components/NaturalViewer.vue'
import { updateUntilDone } from './utils';
import { computed, watch } from '@vue/runtime-core';

export default {
  name: 'App',
  components: {
    NaturalViewer,
  },
  data() {
    return {
      settings: {
        image: {
          height: 100,
        },
        layout: "",
      },
      layoutOptions: [
        { label: "Default", value: "default" },
        { label: "Album", value: "album" },
        { label: "Timeline", value: "timeline" },
        { label: "Wall", value: "wall" },
      ],
      settingsExpanded: false,
      cacheKey: "",
      load: {
        image: 0,
      },
      tasks: null,
      drawer: false,
      simulating: false,
      immersive: false,
      collections: [],
      collectionMenuOpen: false,
    }
  },
  setup(props) {
    const route = useRoute()
    const { data: indexTasks, error: indexTasksError, mutate: indexTasksMutate } = useIndexTasks(() => route.params.collectionId);

    return {
      indexTasks,
      indexTasksError,
      indexTasksMutate,
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
    collection() {
      return this.tasks?.collectionTask.last?.value;
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
    async reindex() {
      await reindexCollection(this.collection?.id);
      updateUntilDone(
        this.indexTasksMutate,
        () => this.indexTasks?.items?.length > 0,
        100
      )
    },
    onTitleClick() {
      this.$bus.emit("home");
    },
    onLoad(load) {
      this.load = { ...this.load, ...load };
    },
    onImmersive(immersive) {
      this.immersive = immersive;
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
  transition: transform 0.2s ease-out;
  transform: translateY(0);
}

button {
  --mdc-theme-primary: black;
}

.top-bar.immersive {
  transform: translateY(-80px);
}

.title {
  cursor: pointer;
}

.files {
  font-size: 0.8em;
  margin-left: 12px;
  color: var(--mdc-theme-text-hint-on-background);
}

.settings-toggle {
  transition: transform 0.5s cubic-bezier(0.22, 1, 0.36, 1);
}

.settings-toggle.expanded {
  transform: rotate(90deg);
}

.collection-menu {
  position: fixed;
}

.settings {
  transition: opacity 0.1s cubic-bezier(0.22, 1, 0.36, 1), transform 0.5s cubic-bezier(0.22, 1, 0.36, 1);
  opacity: 1;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  align-self: start;
  background: var(--mdc-theme-background);
  border-radius: 10px;
  justify-content: center;
  margin-top: -8px;
}

.settings > * {
  margin: 4px 10px 0 10px;
}

.settings.hidden {
  opacity: 0;
  transform: translateX(40px);
}

.size-icons {
  display: flex;
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
  height: calc(100vh - 64px);
}

.viewer.simulating {
  width: 1280px;
  height: 720px;
}

</style>