<template>
  <div class="collection-settings">
    <span class="grow">
      {{ fileCount }} file{{ fileCount == 1 ? "" : "s" }}
      indexed {{ ago(collection?.indexed_at) }}
    </span>
    <ui-button @click="emit('reindex')">Rescan</ui-button>
    <ui-icon-button class="expand" @click="expand = !expand">
      {{ expand ? "expand_less" : "expand_more" }}
    </ui-icon-button>
  </div>
</template>

<script setup>
import { computed, ref, toRefs, watchEffect } from 'vue';
import dateParseISO from 'date-fns/parseISO';
import formatDistance from 'date-fns/formatDistance';

const props = defineProps({
    collection: Object,
    scenes: Array,
    tasks: Array,
});

const emit = defineEmits([
    "reindex",
    "expand",
]);

const expand = ref(false);
watchEffect(() => emit("expand", expand.value))

const {
  collection,
  tasks,
  scenes,
} = toRefs(props);

const fileCount = computed(() => {
  if (collection.value) {
    for (const task of tasks.value) {
      if (task.type != "INDEX") continue;
      if (task.collection_id != collection.value.id) continue;
      return task.done.toLocaleString();
    }
  }
  const scene = scenes.value && scenes.value.length > 0 && scenes.value[0];
  return scene?.file_count !== undefined ?
    scene.file_count.toLocaleString() : 
    null;
});

const ago = (at) => {
  if (!at) return "";
  const date = dateParseISO(at);
  return formatDistance(date, new Date(), { addSuffix: true });
}

</script>

<style scoped>

.collection-settings {
  width: 100%;
  display: flex;
  flex-direction: row;
  align-items: center;
}

.collection-settings :first-child {
  margin-top: 0;
}

.grow {
  flex-grow: 1;
}

</style>