<template>
  <div class="collection-debug">
    <h3>Metadata</h3>
    <p>
      <ui-button @click="emit('reload', 'LOAD_META')">Reindex EXIF</ui-button>
      <ui-button @click="emit('reload', 'LOAD_COLOR')">Reindex colors</ui-button>
      <ui-button @click="emit('reload', 'LOAD_AI')">Reindex AI</ui-button>
      <ui-button @click="emit('reload', 'INDEX_CONTENTS')">Reindex contents</ui-button>
      <task-list :tasks="metadataTasks"></task-list>
    </p>
    
    <h3>Display Layout</h3>
    <table>
      <tr class="rel" v-for="scene in scenes" :key="scene.id">
        <td>
          {{ scene.name }}
        </td>
        <td>
          <i>created {{ ago(scene?.created_at) }}</i>
        </td>
        <td>
          <ui-button @click="recreateEvent.emit(scene)">
            Refresh
          </ui-button>
        </td>
        <td>
          ID: <code>{{ scene.id }}</code>
        </td>
      </tr>
    </table>
  </div>
</template>

<script setup>
import { computed, inject, ref, toRefs } from 'vue';
import dateParseISO from 'date-fns/parseISO';
import formatDistance from 'date-fns/formatDistance';
import TaskList from './TaskList.vue';
import { useEventBus } from '@vueuse/core';

const props = defineProps({
    collection: Object,
    scenes: Array,
    tasks: Array,
});

const {
  collection,
  tasks,
  scenes,
} = toRefs(props);

const metadataTasks = computed(() => {
  return tasks.value?.filter(task => task?.type?.startsWith("LOAD_"));
});

const ago = (at) => {
  if (!at) return "";
  const date = dateParseISO(at);
  return formatDistance(date, new Date(), { addSuffix: true });
}

const recreateEvent = useEventBus("recreate-scene");

const emit = defineEmits([
    "reindex",
    "reload",
]);
</script>

<style scoped>

code {
  color: gray;
}

.rel {
  position: relative;
}

.collection-debug {
  margin-bottom: 16px;
}

.grow {
  flex-grow: 1;
}

h3 {
  margin-bottom: 10px;
}

h4 {
  margin-bottom: 6px;
}

p {
  margin: 0;
}

table {
  border-spacing: 6px;
}

</style>