<template>
  <div class="task-list">
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
        <ui-icon-button
          v-if="task.id"
          icon="close"
          class="stop-button"
          title="Stop task"
          @click="onStopTask(task.id)"
        ></ui-icon-button>
      </ui-item>
    </ui-list>
  </div>
</template>

<script setup>
import { stopTask } from '../api.js';

const props = defineProps({
    tasks: Array
});

async function onStopTask(id) {
  try {
    await stopTask(id);
  } catch (error) {
    console.error(error);
  }
}

</script>

<style scoped>

.task-list {
  display: flex;
  flex-direction: column;
  flex-wrap: wrap;
}

.task-content {
  width: 100%;
}

.stop-button {
  flex-shrink: 0;
}

</style>