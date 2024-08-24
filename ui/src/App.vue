<template>
  <div class="app">
    <ui-top-app-bar
      class="top-bar"
      :class="{ immersive, search: showSearch && searchActive }"
      :fixed="true"
      contentSelector="#content"
    >
      <span class="title">
        <span
          v-if="selecting"
        >
          Selection
          &nbsp;
          <router-link
            :to="{ query: selectSearch }"
          >
            <ui-icon class="inline">
              filter
            </ui-icon>
          </router-link>
        </span>
        <span
          v-else-if="collection"
          @mousedown="collectionExpandedPending = true"
          @click="toggleFocus()"
        >
          <span v-if="selected">
            {{ currentScene?.file_count }} file{{ currentScene?.file_count > 1 ? 's' : '' }} of
          </span>
          {{ collection.name }}
          <ui-icon class="inline">
            {{ collectionExpanded ? 'expand_less' : 'expand_more' }}
          </ui-icon>
        </span>
        <span v-else>{{ title }}</span>
      </span>

      <template #nav-icon>
        <!-- <img src="/favicon-32x32.png" /> -->
        <ui-icon-button @click="goBack()" class="inline">
          {{ collection ? selecting || selected ? 'close' : 'arrow_back' : 'home' }}
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
          :scenes="scenes"
          tabindex="0"
          @focusin="collectionExpanded = true"
          @focusout="collectionExpandedPending = false; collectionExpanded = false"
          @close="collectionExpanded = false"
          @reindex="reindex"
          @reload="reload"
        >
        </collection-panel>

        <search-input
          v-if="showSearch"
          :hide="selected"
          :loading="query.search && scrollScene?.loading"
          :modelValue="selected && !searchActive ? '' : query.search"
          :error="scrollScene?.error"
          @active="searchActive = $event"
          @update:modelValue="onSearch"
        ></search-input>

        <ui-icon-button
          v-if="collection"
          :class="{ toolbarItemClass }"
          @click="showTagEditor = !showTagEditor"
        >
          tag
        </ui-icon-button>

        <ui-dialog
          class="tag-dialog"
          v-model="showTagEditor"
          fullscreen
          maskClosable
        >
          <ui-dialog-title>Tags</ui-dialog-title>
          <ui-dialog-content>
            <tag-editor :tagId="query.select_tag" />
            <ui-dialog-actions>
              <ui-button @click="showTagEditor = false">
                Close
              </ui-button>
            </ui-dialog-actions>
          </ui-dialog-content>
        </ui-dialog>

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
        <a
          v-if="!collection && capabilities?.docs?.supported"
          :href="capabilities?.docs?.url"
        >
          <ui-icon-button
            icon="help_outline"
            class="help"
            :class="{ toolbarItemClass }"
          >
          </ui-icon-button>
        </a>
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
        :fullpage="true"
        :scrollbar="scrollbar"
        @load="onLoad"
        @scene="v => currentScene = v"
        @scenes="v => scenes = v"
        @immersive="onImmersive"
        @tasks="tasks => viewerTasks = tasks"
        @reindex="() => reindex()"
        @title="pageTitle = $event"
      >
      </router-view>
    </div>
  </div>
</template>

<script>
import { createTask, useApi, useTasks } from './api';
import { computed, ref, toRef } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import ExpandButton from './components/ExpandButton.vue'
import SearchInput from './components/SearchInput.vue'
import DisplaySettings from './components/DisplaySettings.vue'
import TaskList from './components/TaskList.vue';
import CollectionPanel from './components/CollectionPanel.vue';
import TagEditor from './components/TagEditor.vue';
import { useEventBus } from '@vueuse/core';

export default {
  name: 'App',
  components: {
    ExpandButton,
    SearchInput,
    DisplaySettings,
    TaskList,
    CollectionPanel,
    TagEditor,
},
  
  props: [
    "collectionId",
  ],

  data() {
    return {
      settingsExpanded: false,
      tasksExpanded: false,
      collectionExpanded: false,
      showTagEditor: false,
      collectionExpandedPending: false,
      load: {
        image: 0,
      },
      drawer: false,
      immersive: false,
      collectionMenuOpen: false,
      scrollbar: null,
      scenes: [],
      currentScene: null,
      viewerTasks: null,
      searchActive: false,
    }
  },
  setup(props) {
    const collectionId = toRef(props, "collectionId");
    const router = useRouter();
    const route = useRoute();
    const query = computed(() => route.query);
    const selecting = computed(() => !!query.value.select_tag);
    const selected = computed(() => {
      const tag = query.value?.search?.split(" ", 2)[0];
      return tag?.startsWith("tag:sys:select:") ? tag : null;
    });
    const selectSearch = computed(() => {
      return {
        ...query.value,
        select_tag: undefined,
        search: `tag:${query.value.select_tag}`,
      }
    });

    const goBack = () => {
      if (selected.value) {
        router.push({
          query: {
            ...query.value,
            search: undefined,
          }
        });
      } else if (selecting.value) {
        router.push({
          query: {
            ...query.value,
            select_tag: undefined,
          }
        });
      } else {
        router.replace({
          path: "/",
        });
      }
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

    const recreateEvent = useEventBus("recreate-scene");

    const pageTitle = ref("");

    const title = computed(() => {
      return pageTitle.value || "Photos";
    });

    return {
      goBack,
      query,
      setQuery,
      selecting,
      selected,
      selectSearch,
      remoteTasks,
      remoteTasksUpdateUntilDone,
      indexTasks,
      indexTasksError,
      indexTasksMutate,
      collection,
      collections,
      capabilities,
      recreateEvent,
      pageTitle,
      title,
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
    scrollScene() {
      return this.scenes?.find(scene => scene.name == "Scroll");
    },
    showSearch() {
      return this.capabilities?.search.supported && this.collection && !this.selecting;
    }
  },
  methods: {
    toggleFocus() {
      if (!this.collectionExpandedPending) return;
      this.$refs.collectionPanel.$el.focus();
      this.collectionExpandedPending = false;
    },
    recreateScene() {
      this.recreateEvent.emit();
    },
    async reindex() {
      await createTask("INDEX_FILES", this.collection?.id);
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
    onSearch(query) {
      if (this.selected) {
        if (!this.searchActive && query == "") {
          this.setQuery({ search: this.selected });
          return;
        }
      }
      this.setQuery({ search: query });
    },
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
  vertical-align: baseline;
  transition: transform 0.2s;
  --mdc-theme-on-primary: rgba(0,0,0,.87);
}

.top-bar :deep(.mdc-button--raised) {
  --mdc-theme-on-primary: #fff;
}

.top-bar.immersive {
  transform: translateY(-80px);
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

.tag-dialog :deep(.mdc-dialog__surface) {
  max-width: 800px !important;
}

button {
  --mdc-theme-primary: black;
}


.title {
  cursor: pointer;
}

.files {
  font-size: 0.8em;
  vertical-align: bottom;
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

</style>