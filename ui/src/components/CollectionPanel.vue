<template>
  <div class="collection-panel">
    <collection-settings
      :collection="collection"
      :scene="scene"
      :tasks="tasks"
      @reindex="emit('reindex')"
      @reload="emit('reload', $event)"
      @recreate-scene="emit('recreateScene')"
      @simulate="emit('simulate')"
    >
    </collection-settings>
    
    <ui-divider></ui-divider>

    <ui-list
      class="list"
      v-if="collections?.length > 0"
    >
      <OverlayScrollbars class="scrollbar">
        <router-link
          v-for="c in collections"
          :key="c.id"
          class="no-decoration"
          :to="'/collections/' + c.id"
          @click="emit('close')"
        >
          <ui-item
            :active="c.id == collection?.id"
          >
              {{ c.name }}
          </ui-item>
        </router-link>
      </OverlayScrollbars>
    </ui-list>
  </div>
</template>

<script setup>
import CollectionSettings from './CollectionSettings.vue';
import TaskList from './TaskList.vue';
import { onMounted, ref, watch } from 'vue';
import { OverlayScrollbarsComponent as OverlayScrollbars } from "overlayscrollbars-vue";

const props = defineProps({
    collections: Array,
    collection: Object,
    scene: Object,
    tasks: Array,
});

console.log(props)

const emit = defineEmits([
    "close",
    "reindex",
    "reload",
    "recreateScene",
    "simulate",
]);

</script>

<style scoped>

.collection-panel {
  max-width: 400px;
  background: var(--mdc-theme-background);
  border-radius: 10px;
  padding: 0 16px 16px 16px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}

.no-decoration {
  text-decoration: none;
}

.tight {
  margin-bottom: 0;
}

.list {
  flex-basis: 600px;
}

.scrollbar {
  height: 100%;
}

</style>