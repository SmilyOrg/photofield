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
            <span v-if="currentScene?.file_count">
              {{ currentScene?.file_count }} file{{ currentScene?.file_count > 1 ? 's' : '' }} 
            </span>
            <span v-else>
              Files
            </span>
            of
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
          v-if="!immersive"
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
          :loading="query.search && scrollScene?.loading"
          :modelValue="selected && !searchActive ? undefined : query.search"
          :error="scrollScene?.error"
          @active="searchActive = $event"
          @update:modelValue="onSearch"
        ></search-input>

        <ui-icon-button
          v-if="collection && capabilities?.tags?.supported && selecting"
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

        <ui-card class="tasks" :class="{ hidden: !tasksExpanded, toolbarItemClass }">
          <div class="empty" v-if="!tasks?.length">
            No background tasks running.
          </div>
          <task-list
            :tasks="tasks"
          ></task-list>
        </ui-card>
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
        <color-mode-switch
          v-else
          :class="{ toolbarItemClass }"
        ></color-mode-switch>
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
import { computed, ref, toRef, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import ExpandButton from './components/ExpandButton.vue'
import SearchInput from './components/SearchInput.vue'
import DisplaySettings from './components/DisplaySettings.vue'
import TaskList from './components/TaskList.vue';
import CollectionPanel from './components/CollectionPanel.vue';
import TagEditor from './components/TagEditor.vue';
import ColorModeSwitch from './components/ColorModeSwitch.vue';
import { useColorMode, useEventBus } from '@vueuse/core';
import { useHead } from '@unhead/vue';

export default {
  name: 'App',
  components: {
    ExpandButton,
    SearchInput,
    DisplaySettings,
    TaskList,
    CollectionPanel,
    TagEditor,
    ColorModeSwitch,
  },
  
  props: [
    "collectionId",
  ],

  data() {
    return {
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
    const settingsExpanded = ref(false);
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
      if (selecting.value) {
        router.push({
          query: {
            ...query.value,
            select_tag: undefined,
          },
          hash: route.hash,
        });
      } else if (selected.value) {
        router.push({
          query: {
            ...query.value,
            search: undefined,
          },
          hash: route.hash,
        });
      } else {
        router.replace({
          name: "home",
        });
      }
    }

    const setQuery = (patch) => {
      settingsExpanded.value = false;
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
      router.push({ query, hash: route.hash });
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

    const colorMode = useColorMode();
    const themeColor = computed(() => {
      return colorMode.value == "dark" ? "#222" : "#fff";
    });
    useHead({
      meta: [
        { name: "theme-color", content: themeColor },
      ],
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
      settingsExpanded,
    }
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
      this.setQuery({ search: query, f: undefined });
    },
  }
}
</script>

<style>
html.light {
  --mdc-theme-primary: #6782ff;
  --mdc-theme-surface: #f5f5f5;
}
html.dark {
  --mdc-theme-background: #222;
  --mdc-theme-on-background: white;
  --mdc-theme-text-primary-on-background: #fff;

  --mdc-theme-primary: #6782ff;
  --mdc-theme-secondary: #018786;
  --mdc-theme-surface: #333;
  --mdc-theme-error: #b00020;
  --mdc-theme-on-primary: #fff;
  --mdc-theme-on-secondary: #fff;
  --mdc-theme-on-surface: #fff;
  --mdc-theme-on-error: #fff;
  --mdc-theme-text-primary-on-background: rgba(100%,100%,100%,.87);
  --mdc-theme-text-secondary-on-background: rgba(100%,100%,100%,.54);
  --mdc-theme-text-hint-on-background: rgba(100%,100%,100%,.38);
  --mdc-theme-text-disabled-on-background: rgba(100%,100%,100%,.38);
  --mdc-theme-text-icon-on-background: rgba(100%,100%,100%,.38);
  --mdc-theme-text-primary-on-light: rgba(100%,100%,100%,.87);
  --mdc-theme-text-secondary-on-light: rgba(100%,100%,100%,.54);
  --mdc-theme-text-hint-on-light: rgba(100%,100%,100%,.38);
  --mdc-theme-text-disabled-on-light: rgba(100%,100%,100%,.38);
  --mdc-theme-text-icon-on-light: rgba(100%,100%,100%,.38);
  --mdc-theme-text-primary-on-dark: hsla(0,0%,100%,.87);
  --mdc-theme-text-secondary-on-dark: hsla(0,0%,100%,.7);
  --mdc-theme-text-hint-on-dark: hsla(0,0%,100%,.5);
  --mdc-theme-text-disabled-on-dark: hsla(0,0%,100%,.5);
  --mdc-theme-text-icon-on-dark: hsla(0,0%,100%,.5);

  --mdc-ripple-color: #fff;

  color: var(--mdc-theme-text-primary-on-background);
  background-color: var(--mdc-theme-background);
}

html a:active {
  color: var(--mdc-theme-primary);
}

html.dark a {
  color: var(--mdc-theme-text-primary-on-background);
}
html.dark a:visited {
  color: var(--mdc-theme-text-secondary-on-background);
}

html.dark .mdc-skeleton--active .mdc-skeleton-avatar, html.dark .mdc-skeleton--active .mdc-skeleton-paragraph > li, html.dark .mdc-skeleton--active .mdc-skeleton-title {
  background-image: linear-gradient(90deg,#333 25%,#2c2c2c 37%,#333 63%);
}

html .multiselect__tags, html .multiselect__tags input, html .multiselect__content-wrapper, html .multiselect__input, html .multiselect__single {
  color: var(--mdc-theme-on-surface);
  border-color: var(--mdc-theme-surface);
  background-color: var(--mdc-theme-surface);
}

html .multiselect__input::placeholder {
  color: var(--mdc-theme-text-hint-on-surface);
}

html .multiselect__spinner {
  background-color: var(--mdc-theme-surface);
}

/* RouterLink inside <ui-item> */
  .mdc-deprecated-list-item > a {
  width: 100%;
  height: 100%;
  align-content: center;
  text-decoration: none;
}

</style>

<style scoped>

.top-bar :deep(.mdc-select--filled:not(.mdc-select--disabled) .mdc-select__anchor) {
  background-color: var(--mdc-theme-surface);
  color: var(--mdc-theme-on-surface);
}

.top-bar :deep(.mdc-select:not(.mdc-select--disabled) .mdc-floating-label) {
  color: var(--mdc-theme-text-secondary-on-background);
}

.top-bar :deep(.mdc-select--filled:not(.mdc-select--disabled) .mdc-select__selected-text) {
  color: var(--mdc-theme-text-primary-on-background);
}

.top-bar :deep(.mdc-dialog .mdc-dialog__title), .top-bar :deep(.mdc-dialog .mdc-dialog__content) {
  color: var(--mdc-theme-text-primary-on-background);
}

.top-bar :deep(.mdc-checkbox .mdc-checkbox__native-control:enabled:not(:checked):not(:indeterminate):not([data-indeterminate="true"]) ~ .mdc-checkbox__background) {
  border-color: var(--mdc-theme-primary-on-background);
}

.top-bar :deep(.mdc-linear-progress__bar-inner) {
  border-color: var(--mdc-theme-text-primary-on-background);
}

.top-bar :deep(.mdc-text-field input) {
  color: var(--mdc-theme-text-primary-on-background);
}

.top-bar :deep(.mdc-text-field input::placeholder) {
  color: var(--mdc-theme-text-secondary-on-background);
}

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
  border-radius: 10px;
  padding: 0px;
}

.tasks .empty {
  padding: 16px 16px 0 16px;
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
  background-color: var(--mdc-theme-background);
  color: var(--mdc-theme-text-primary-on-background);
  vertical-align: baseline;
  transition: transform 0.2s;
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

.tag-dialog :deep(.mdc-dialog__surface) {
  max-width: 800px !important;
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