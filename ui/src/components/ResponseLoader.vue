<template>
  <div
    v-if="loading || error"
    class="response"
  >
    <ui-spinner
      v-if="!data && loading && !error"
      class="spinner"
      active
    >
    </ui-spinner>
    <error-bar
      v-if="lastError"
      title="Connection error"
      title-loading="Connecting..."
      :subtitle="errorTime && ago"
      :detail="error"
      :loading="loading"
      @click="retry"
    ></error-bar>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue';
import { useTimeAgo } from '@vueuse/core';
import ErrorBar from './ErrorBar.vue';

const { response } = defineProps({
  response: {
    type: Object,
    required: true,
  },
});

const {
  isValidating: loading,
  mutate: retry,
  data,
  error,
  errorTime
} = response;

const lastError = ref(null);

watch(data, () => {
  if (data.value) {
    lastError.value = null;
  }
});

watch(error, () => {
  if (error.value) {
    lastError.value = error.value;
  }
});

const ago = useTimeAgo(errorTime, {
  showSecond: true,
  updateInterval: 1000,
});

</script>

<style scoped>

.response {
  position: absolute;
  z-index: 10;
}

.spinner {
  --size: 48px;
  position: fixed;
  width: var(--size);
  height: var(--size);
  top: calc(50% - var(--size) / 2);
  left: calc(50% - var(--size) / 2);
  box-sizing: border-box;
}

</style>
