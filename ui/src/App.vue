<template>
  <div class="app">
    <ui-top-app-bar
      class="top-bar"
      :class="{ immersive, search: searchActive }"
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

        <search-input
          v-if="capabilities?.search.supported"
          :loading="query.search && scene?.loading"
          :modelValue="query.search"
          :error="scene?.error"
          @active="searchActive = $event"
          @update:modelValue="setQuery('search', $event)"
        ></search-input>

        <div class="tasks" :class="{ hidden: !tasksExpanded, toolbarItemClass }">
          <span v-if="!tasks?.length">
            No background tasks running.
          </span>
          <ui-list :type="2" :nonInteractive="true">
            <ui-item
              v-for="task in tasks"
              :key="task.id"
            >
              <ui-item-text-content class="task-content" v-if="task.pending !== undefined && task.done !== undefined">
                <ui-item-text1>{{ task.name }}</ui-item-text1>
                <ui-item-text2>{{ task.done }} / {{ task.done + task.pending }} files</ui-item-text2>
                <ui-progress
                  class="task-progress"
                  :progress="task.done / (task.done + task.pending)"
                ></ui-progress>
              </ui-item-text-content>
              <ui-item-text-content class="task-content" v-else-if="task.pending !== undefined">
                <ui-item-text1>{{ task.name }}</ui-item-text1>
                <ui-item-text2>{{ task.pending }} remaining</ui-item-text2>
                <ui-progress
                  class="task-progress"
                  active
                ></ui-progress>
              </ui-item-text-content>
              <ui-item-text-content class="task-content" v-else-if="task.done !== undefined">
                <ui-item-text1>{{ task.name }}</ui-item-text1>
                <ui-item-text2>{{ task.done }} files</ui-item-text2>
                <ui-progress
                  class="task-progress"
                  active
                ></ui-progress>
              </ui-item-text-content>
            </ui-item>
          </ui-list>
        </div>
        <div class="settings" :class="{ hidden: !settingsExpanded, toolbarItemClass }">
          <ui-select
            :modelValue="query.layout"
            @update:modelValue="setQuery('layout', $event)"
            :options="layoutOptions"
          >
            Layout
          </ui-select>
          <div class="size-icons">
            <ui-icon-button
              icon="photo_size_select_small"
              :class="{ active: query.image_height == '30' }"
              @click="setQuery('image_height', 30)"
              outlined
            >
            </ui-icon-button>
            <ui-icon-button
              icon="photo_size_select_large"
              :class="{ active: query.image_height == '100' }"
              @click="setQuery('image_height', query.image_height == 100 ? undefined : 100)"
              outlined
            >
            </ui-icon-button>
            <ui-icon-button
              icon="photo_size_select_actual"
              :class="{ active: query.image_height == '300' }"
              @click="setQuery('image_height', 300)"
              outlined
            >
            </ui-icon-button>
          </div>
          <expand-button
            :expanded="settingsExtraExpanded"
            @click="settingsExtraExpanded = !settingsExtraExpanded"
          ></expand-button>
          <div v-if="settingsExtraExpanded">
            <ui-form-field>
              <ui-checkbox v-model="settings.debug.overdraw"></ui-checkbox>
              <label>Debug Overdraw</label>
            </ui-form-field>
            <ui-form-field>
              <ui-checkbox v-model="settings.debug.thumbnails"></ui-checkbox>
              <label>Debug Thumbnails</label>
            </ui-form-field>
          </div>
        </div>
        <ui-icon-button
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
    <ui-drawer class="sidebar" type="modal" nav-id="menu" v-model="drawer">
      <template v-if="collection">
        <ui-drawer-header>
          <ui-drawer-title>{{ collection?.name }}</ui-drawer-title>
          <ui-drawer-subtitle>
            {{ fileCount }} files
          </ui-drawer-subtitle>
        </ui-drawer-header>
        <ui-button @click="reindex()">Reindex files</ui-button>
        <expand-button
          :expanded="collectionExpanded"
          @click="collectionExpanded = !collectionExpanded"
        ></expand-button>
        <template v-if="collectionExpanded">
          <ui-button @click="recreateScene()">
            Reload scene
          </ui-button>
          <ui-button @click="reload('LOAD_META')">Reload metadata</ui-button>
          <ui-button @click="reload('LOAD_COLOR')">Reload colors</ui-button>
          <ui-button @click="reload('LOAD_AI')">Reload ai</ui-button>
          <ui-button @click="simulate()">
            Simulate
          </ui-button>
        </template>
      </template>
      <ui-divider></ui-divider>
      <ui-drawer-header>
        <ui-drawer-title>Photos</ui-drawer-title>
        <ui-drawer-subtitle>
          {{ collections?.length }} collections
        </ui-drawer-subtitle>
      </ui-drawer-header>
      <ui-drawer-content v-if="collections?.length > 0">
        <ui-list>
          <router-link
            v-for="c in collections"
            :key="c.id"
            class="collection"
            :to="'/collections/' + c.id"
            @click="drawer = false"
          >
            <ui-item
              :active="c.id == collection?.id"
            >
                {{ c.name }}
            </ui-item>
          </router-link>
        </ui-list>
      </ui-drawer-content>
    </ui-drawer>
    <div id="content">
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
        @tasks="tasks => viewerTasks = tasks"
        @reindex="() => reindex()"
      >
      </router-view>
    </div>
  </div>
</template>

<script>
import { createTask, useApi, useTasks } from './api';
import NaturalViewer from './components/NaturalViewer.vue'
import ExpandButton from './components/ExpandButton.vue'
import SearchInput from './components/SearchInput.vue'
import { computed, toRef } from 'vue';
import { useRoute, useRouter } from 'vue-router';

export default {
  name: 'App',
  components: {
    NaturalViewer,
    ExpandButton,
    SearchInput,
  },
  
  props: [
    "collectionId",
  ],

  data() {
    return {
      settings: {
        debug: {
          overdraw: false,
          thumbnails: false,
        },
      },
      settingsExpanded: false,
      settingsExtraExpanded: false,
      tasksExpanded: false,
      collectionExpanded: false,
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

    const setQuery = (key, value) => {
      if (value == "" || (key == "layout" && value == "DEFAULT")) {
        value = undefined;
      }
      const query = Object.assign({}, route.query);
      query[key] = value;
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
    fileCount() {
      if (this.collection) {
        for (const task of this.tasks) {
          if (task.type != "INDEX") continue;
          if (task.collection_id != this.collection.id) continue;
          return task.done.toLocaleString();
        }
      }
      return this.scene?.file_count !== undefined ?
        this.scene.file_count.toLocaleString() : 
        null;
    },
  },
  methods: {
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
  background-color: white;
  --mdc-theme-on-primary: rgba(0,0,0,.87);
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

.top-bar .mdc-select {
  --mdc-theme-primary: white; 
}

.collection {
  text-decoration: none;
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

.tasks {
  transition: opacity 0.1s cubic-bezier(0.22, 1, 0.36, 1), transform 0.5s cubic-bezier(0.22, 1, 0.36, 1);
  opacity: 1;
  position: absolute;
  display: flex;
  flex-direction: column;
  align-items: center;
  flex-wrap: wrap;
  align-self: start;
  background: var(--mdc-theme-background);
  border-radius: 10px;
  justify-content: center;
  margin-top: 50px;
  padding: 0px 10px;
}

.tasks.hidden {
  opacity: 0;
  pointer-events: none;
  transform: translateX(40px);
}

.task-content {
  width: 100%;
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