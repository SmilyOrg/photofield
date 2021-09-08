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
      <span class="title">
        <router-link class="title-home" to="/">
          Photos
        </router-link>
        <span v-if="collection" class="title-collection" @click="onTitleClick()">
          <ui-icon>chevron_right</ui-icon> {{ collection.name }}
        </span>
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
    <ui-drawer class="sidebar" type="modal" nav-id="menu" v-model="drawer">
      <template v-if="collection">
        <ui-drawer-header>
          <ui-drawer-title>{{ collection.name }}</ui-drawer-title>
          <ui-drawer-subtitle>
            {{ indexTasks?.items[0]?.count || fileCount }} files
          </ui-drawer-subtitle>
        </ui-drawer-header>
        <ui-button @click="reindex()">Reindex Collection</ui-button>
        <ui-button @click="simulate()">
          Simulate
        </ui-button>
        <ui-button @click="refreshCache()">
          Refresh Cache
        </ui-button>
      </template>
      <ui-divider></ui-divider>
      <ui-drawer-header>
        <ui-drawer-title>Photos</ui-drawer-title>
        <ui-drawer-subtitle>
          {{ collections?.length }} collections
        </ui-drawer-subtitle>
      </ui-drawer-header>
      <ui-drawer-content v-if="collections?.length > 0">
        <ui-nav>
          <ui-nav-item
            v-for="c in collections"
            :key="c.id"
            :href="'/collections/' + c.id"
            :active="c.id == collection?.id"
          >
            {{ c.name }}
          </ui-nav-item>
        </ui-nav>
      </ui-drawer-content>
    </ui-drawer>
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
        :class="{ simulating }"
        :fullpage="true"
        :scrollbar="scrollbar"
        @load="onLoad"
        @scene="v => scene = v"
        @immersive="onImmersive"
      >
      </router-view>
    </div>
  </div>
</template>

<script>
import { reindexCollection, useApi } from './api';
import NaturalViewer from './components/NaturalViewer.vue'
import { updateUntilDone } from './utils';
import { computed, toRef } from 'vue';

export default {
  name: 'App',
  components: {
    NaturalViewer,
  },
  
  props: [
    "collectionId",
  ],

  data() {
    return {
      settings: {
        image: {
          height: 100,
        },
        layout: "",
      },
      layoutOptions: [
        { label: "Album", value: "ALBUM" },
        { label: "Timeline", value: "TIMELINE" },
        { label: "Wall", value: "WALL" },
      ],
      settingsExpanded: false,
      load: {
        image: 0,
      },
      drawer: false,
      simulating: false,
      immersive: false,
      collectionMenuOpen: false,
      scrollbar: null,
      scene: null,
    }
  },
  setup(props) {
    const collectionId = toRef(props, "collectionId");

    const { data: indexTasks, error: indexTasksError, mutate: indexTasksMutate } = useApi(
      () => collectionId.value && `/index-tasks?collection_id=${collectionId.value}`
    );

    const { items: collections } = useApi(() => "/collections");
    const { data: fetchedCollection } = useApi(
      () => collectionId.value && `/collections/${collectionId.value}`
    );

    const collection = computed(() => collectionId.value && fetchedCollection.value);

    return {
      indexTasks,
      indexTasksError,
      indexTasksMutate,
      collection,
      collections,
    }
  },
  async mounted() {
    this.scrollbar = OverlayScrollbars(document.querySelectorAll('body'), {
      className: "os-theme-minimal-dark",
      scrollbars: {
        clickScrolling: true,
      },
    });
    this.scrollbar.addExt("timeline");
  },
  watch: {
    collection(newCollection, oldCollection) {
      if (newCollection && newCollection?.id != oldCollection?.id) {
        this.settings.layout = newCollection.layout;
      }
    },
  },
  computed: {
    loading() {
      // TODO reimplement?
      return false;
    },
    fileCount() {
      return this.scene?.photo_count !== undefined ?
        this.scene.photo_count.toLocaleString() : 
        null;
    }
  },
  methods: {
    refreshCache() {
      // TODO: reimplement
    },
    async reindex() {
      await reindexCollection(this.collection?.id);
      await updateUntilDone(
        this.indexTasksMutate,
        () => this.indexTasks?.items?.length > 0,
        100
      );
      this.refreshCache();
    },
    onTitleClick() {
      this.$bus.emit("home");
    },
    onLoad(load) {
      this.load = { ...this.load, ...load };
    },
    onImmersive(immersive) {
      this.immersive = immersive;
      if (immersive) {
        this.settingsExpanded = false;
      }
      this.scrollbar?.options({
        scrollbars: {
          visibility: immersive ? "hidden" : "auto",
        },
      })
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
      this.$bus.emit("simulate-run");
    }
  }
}
</script>

<style scoped>

.title-collection i {
  vertical-align: sub;
}

.title-home {
  text-decoration: none;
  color: inherit;
}

.sidebar button {
    padding: 20px 0;
    margin: 2px 0;
}

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
  width: max-content;
}

.settings {
  transition: opacity 0.1s cubic-bezier(0.22, 1, 0.36, 1), transform 0.5s cubic-bezier(0.22, 1, 0.36, 1);
  opacity: 1;
  position: absolute;
  width: min-content;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  align-self: start;
  background: var(--mdc-theme-background);
  border-radius: 10px;
  justify-content: center;
  margin-top: -8px;
  margin-right: 80px;
  padding-bottom: 4px;
}

.settings > * {
  margin: 4px 10px 0 10px;
}

.settings.hidden {
  opacity: 0;
  pointer-events: none;
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