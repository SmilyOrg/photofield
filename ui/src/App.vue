<template>
  <div class="app">
    <ui-top-app-bar
      class="top-bar"
      :class="{ immersive, search: searchActive }"
      :fixed="true"
      contentSelector="#content"
    >
      <span class="title">
        <span v-if="!collection">Photos</span>
        <span
          v-if="collection"
          ref="title"
          @mousedown="collectionExpandedPending = true"
          @click="toggleFocus()"
        >
          {{ collection.name }}
          <ui-icon class="inline">
            {{ collectionExpanded ? 'expand_less' : 'expand_more' }}
          </ui-icon>
        </span>
      </span>

      <template #nav-icon>
        <!-- <img src="/favicon-32x32.png" /> -->
        <ui-icon-button @click="goHome()" class="inline">
          {{ collection ? 'arrow_back' : 'home' }}
        </ui-icon-button>
      </template>

      <template #toolbar="{ toolbarItemClass }">

        <collection-panel
          class="collection-panel"
          :class="{ hidden: !collectionExpanded }"
          ref="collectionPanel"
          :collections="collections"
          :collection="collection"
          :tasks="tasks"
          :scene="scene"
          tabindex="0"
          @focusin="collectionExpanded = true"
          @focusout="collectionExpandedPending = false; collectionExpanded = false"
          @close="collectionExpanded = false"
          @reindex="reindex"
          @reload="reload"
          @recreate-scene="recreateScene"
          @simulate="simulate"
        >
        </collection-panel>

        <search-input
          v-if="capabilities?.search.supported"
          :loading="query.search && scene?.loading"
          :modelValue="query.search"
          :error="scene?.error"
          @active="searchActive = $event"
          @update:modelValue="setQuery({ search: $event })"
        ></search-input>

        <div class="tasks" :class="{ hidden: !tasksExpanded, toolbarItemClass }">
          <span class="empty" v-if="!tasks?.length">
            No background tasks running.
          </span>
          <task-list
            :tasks="tasks"
          ></task-list>
        </div>
        <div class="settings" :class="{ hidden: !collection || !settingsExpanded, toolbarItemClass }">
          <display-settings
            :query="query"
            @query="setQuery($event)"
          ></display-settings>
        </div>
        <ui-icon-button
          v-if="collection"
          icon="settings"
          class="settings-toggle"
          :class="{ expanded: settingsExpanded, toolbarItemClass }"
          @click="settingsExpanded = !settingsExpanded"
        >
        </ui-icon-button>
        <ui-icon-button
          @click="tasksExpanded = !tasksExpanded"
        >
          <ui-spinner
            class="small-spinner"
            size="small"
            :active="tasksProgress == -1"
            :progress="(tasksProgress >= 0 && tasksProgress) || 0"
            :closed="tasksProgress === null"
            :class="toolbarItemClass"
          ></ui-spinner>
        </ui-icon-button>
      </template>
    </ui-top-app-bar>
    <div id="content">
      <router-view
        class="viewer"
        ref="viewer"
        :class="{ simulating }"
        :fullpage="true"
        :scrollbar="scrollbar"
        @load="onLoad"
        @scene="v => scene = v"
        @immersive="onImmersive"
        @tasks="tasks => viewerTasks = tasks"
        @reindex="() => reindex()"
      >
      </router-view>
    </div>
  </div>
</template>

<script>
import { createTask, useApi, useTasks } from './api';
import { computed, toRef } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import NaturalViewer from './components/NaturalViewer.vue'
import ExpandButton from './components/ExpandButton.vue'
import SearchInput from './components/SearchInput.vue'
import DisplaySettings from './components/DisplaySettings.vue'
import TaskList from './components/TaskList.vue';
import CollectionPanel from './components/CollectionPanel.vue';

export default {
  name: 'App',
  components: {
    NaturalViewer,
    ExpandButton,
    SearchInput,
    DisplaySettings,
    TaskList,
    CollectionPanel,
},
  
  props: [
    "collectionId",
  ],

  data() {
    return {
      settingsExpanded: false,
      tasksExpanded: false,
      collectionExpanded: false,
      collectionExpandedPending: false,
      load: {
        image: 0,
      },
      drawer: false,
      simulating: false,
      immersive: false,
      collectionMenuOpen: false,
      scrollbar: null,
      scene: null,
      viewerTasks: null,
      searchActive: false,
    }
  },
  setup(props) {
    const collectionId = toRef(props, "collectionId");
    const layoutOptions = computed(() => {
      return [
        { label: `Default`, value: "DEFAULT" },
        { label: "Album", value: "ALBUM" },
        { label: "Timeline", value: "TIMELINE" },
        { label: "Wall", value: "WALL" },
      ]
    })

    const router = useRouter();
    const route = useRoute();
    const query = computed(() => route.query);

    const goHome = () => {
      router.push("/");
    }

    const setQuery = (patch) => {
      const query = Object.assign({}, route.query);
      Object.assign(query, patch);
      for (const key in patch) {
        if (Object.hasOwnProperty.call(patch, key)) {
          const value = patch[key];
          if (value == "" || (key == "layout" && value == "DEFAULT")) {
            query[key] = undefined;
          }
        }
      }
      router.push({ query });
    }

    const { items: indexTasks, error: indexTasksError, mutate: indexTasksMutate } = useApi(
      () => collectionId.value && `/tasks?type=INDEX&collection_id=${collectionId.value}`
    );

    const { items: remoteTasks, updateUntilDone: remoteTasksUpdateUntilDone } = useTasks();

    const { items: collections } = useApi(() => "/collections");
    const { data: fetchedCollection } = useApi(
      () => collectionId.value && `/collections/${collectionId.value}`
    );

    const { data: capabilities } = useApi(() => "/capabilities");

    const collection = computed(() => collectionId.value && fetchedCollection.value);

    return {
      goHome,
      query,
      setQuery,
      layoutOptions,
      remoteTasks,
      remoteTasksUpdateUntilDone,
      indexTasks,
      indexTasksError,
      indexTasksMutate,
      collection,
      collections,
      capabilities,
    }
  },
  async mounted() {
    this.scrollbar = OverlayScrollbars(document.querySelectorAll('body'), {
      className: "os-theme-minimal-dark",
      scrollbars: {
        clickScrolling: true,
      },
    });
  },
  computed: {
    tasks() {
      const tasks = [];
      if (this.viewerTasks) {
        tasks.push(...this.viewerTasks);
      }
      if (this.remoteTasks?.length > 0) {
        for (const task of this.remoteTasks) {
          tasks.push(task);
        }
      }
      if (this.load.image.inProgress) {
        tasks.push({
          id: "image-load",
          name: "Downloading",
          pending: this.load.image.inProgress,
        });
      }
      return tasks;
    },
    tasksProgress() {
      let done = 0;
      let pending = 0;
      for (const task of this.tasks) {
        if (task.done !== undefined) done += task.done;
        if (task.pending !== undefined) pending += task.pending;
      }
      if (done > 0 && pending > 0) {
        return done / (done + pending);
      }
      if (done > 0 || pending > 0) {
        return -1;
      }
      return null;
    },
  },
  methods: {
    toggleFocus() {
      if (!this.collectionExpandedPending) return;
      this.$refs.collectionPanel.$el.focus();
      this.collectionExpandedPending = false;
    },
    recreateScene() {
      this.$bus.emit("recreate-scene");
    },
    async reindex() {
      await createTask("INDEX", this.collection?.id);
      await this.remoteTasksUpdateUntilDone();
      this.recreateScene();
    },
    async reload(type) {
      await createTask(type, this.collection?.id);
      this.drawer = false;
      await this.remoteTasksUpdateUntilDone();
      this.recreateScene();
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
        this.tasksExpanded = false;
      }
      this.scrollbar?.options({
        scrollbars: {
          visibility: immersive ? "hidden" : "auto",
        },
      })
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

.favicon {
  border: 1px solid #e5e5e5;
  border-radius: 6px;
}

.inline {
  vertical-align: sub;
}

.collection-panel {
  opacity: 1;
  position: absolute;
  top: 44px;
  left: 0px;
  width: 100%;
  max-height: calc(100vh - 120px);
  transition: opacity 0.1s cubic-bezier(0.22, 1, 0.36, 1), transform 0.5s cubic-bezier(0.22, 1, 0.36, 1);
  outline: none;
  box-sizing: border-box;
}

@media screen and (min-width: 600px) {
  .collection-panel {
    left: 44px;
  }
}

.collection-panel.hidden {
  opacity: 0;
  pointer-events: none;
  transform: translateY(-2px);
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
  top: 0px;
  right: 50px;
}

.settings.hidden {
  opacity: 0;
  pointer-events: none;
  transform: translateX(40px);
}

.tasks {
  transition: opacity 0.1s cubic-bezier(0.22, 1, 0.36, 1), transform 0.5s cubic-bezier(0.22, 1, 0.36, 1);
  opacity: 1;
  position: absolute;
  top: 55px;
  right: 10px;
  z-index: 10;
  background: var(--mdc-theme-background);
  border-radius: 10px;
  padding: 0 10px;
}

.tasks.hidden {
  opacity: 0;
  pointer-events: none;
  transform: translateX(40px);
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
  background-color: white;
  --mdc-theme-on-primary: rgba(0,0,0,.87);
  vertical-align: baseline;
}

.top-bar :deep(.mdc-top-app-bar__title) {
  padding-left: 0px;
}

.top-bar :deep(.mdc-top-app-bar__section--align-start) {
  transition: max-width 0.1s, padding-left 0.2s, padding-right 0.2s;
  max-width: 100%;
}

.top-bar.search :deep(.mdc-top-app-bar__section--align-start) {
  max-width: 0;
  padding-left: 0;
  padding-right: 0;
  overflow: hidden;
}

.top-bar :deep(.mdc-select) {
  --mdc-theme-primary: white; 
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


.size-icons {
  display: flex;
}

.size-icons button:before {
  border-radius: 0;
  opacity: 0.03;
}

.size-icons button {
  opacity: 0.3;
}

.size-icons button.active {
  opacity: 1;
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

.task-progress {
  --mdc-theme-primary: var(--mdc-theme-on-primary);
}

.viewer {
  height: calc(100vh - 64px);
}

.viewer.simulating {
  width: 1280px;
  height: 720px;
}

</style>