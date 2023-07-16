<template>
  <div class="tasks" :class="{ hidden: !expanded, toolbarItemClass }">
    <span class="empty" v-if="!tasks?.length">
      No background tasks running.
    </span>
    <task-list
      :tasks="tasks"
    ></task-list>
  </div>
  <ui-icon-button
    @click="expanded = !expanded"
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

<script setup>
import { ref, toRefs } from 'vue';

import TaskList from './TaskList.vue';

const props = defineProps([
  "toolbarItemClass",
  "tasksProgress",
  "tasks",
]);

const {
  toolbarItemClass,
} = toRefs(props);

const expanded = ref(false);


</script>

<style scoped>
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

.hidden {
  opacity: 0;
  pointer-events: none;
  transform: translateX(40px);
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