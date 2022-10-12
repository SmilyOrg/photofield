<template>
  <div class="collection-settings" :class="{ 'bottom-space': extra }">
    
    <p class="top-row">
      <span class="grow">{{ fileCount }} file{{ fileCount == 1 ? "" : "s" }} indexed {{ ago(collection?.indexed_at) }}</span>
      <ui-button @click="emit('reindex')">Rescan</ui-button>
      <ui-icon-button class="expand" @click="extra = !extra">
        {{ extra ? "expand_less" : "expand_more" }}
      </ui-icon-button>
    </p>

    <template v-if="extra">
      <h3>Metadata</h3>
      <p>
        <ui-button @click="emit('reload', 'LOAD_META')">Reindex EXIF</ui-button>
        <ui-button @click="emit('reload', 'LOAD_COLOR')">Reindex colors</ui-button>
        <ui-button @click="emit('reload', 'LOAD_AI')">Reindex AI</ui-button>
        <task-list :tasks="metadataTasks"></task-list>
      </p>
      
      <h3>Display Layout</h3>
      <p>
        Dimensions: {{ Math.round(scene?.bounds.w) }} &times; {{ Math.round(scene?.bounds.h) }} px<br>
        Created {{ ago(scene?.created_at) }}
        <ui-button @click="emit('recreateScene')">
          Refresh
        </ui-button>
      </p>
    
      <h3>Benchmark</h3>
      <ui-button @click="emit('simulate')">
        Simulate scrolling
      </ui-button>
    </template>
  </div>
</template>

<script setup>
import { computed, ref, toRefs } from 'vue';
import dateParseISO from 'date-fns/parseISO';
import formatDistance from 'date-fns/formatDistance';
import TaskList from './TaskList.vue';

const props = defineProps({
    collection: Object,
    scene: Object,
    tasks: Array,
});

const { collection, tasks, scene } = toRefs(props);
const extra = ref(false);

const fileCount = computed(() => {
    if (collection.value) {
      for (const task of tasks.value) {
        if (task.type != "INDEX") continue;
        if (task.collection_id != collection.value.id) continue;
        return task.done.toLocaleString();
      }
    }
    return scene.value?.file_count !== undefined ?
      scene.value.file_count.toLocaleString() : 
      null;
});

const metadataTasks = computed(() => {
  return tasks.value?.filter(task => task?.type?.startsWith("LOAD_"));
});

const ago = (at) => {
  if (!at) return "";
  const date = dateParseISO(at);
  return formatDistance(date, new Date(), { addSuffix: true });
}

const emit = defineEmits([
    "reindex",
    "reload",
    "recreateScene",
    "simulate",
]);
</script>

<style scoped>

.collection-settings {
  width: 100%;
}

.collection-settings :first-child {
  margin-top: 0;
}

.bottom-space {
  margin-bottom: 20px;
}

.grow {
  flex-grow: 1;
}

h3 {
  margin-bottom: 10px;
}

p {
  margin: 0;
}

.top-row {
  display: flex;
  flex-direction: row;
  align-items: center;
}

</style>