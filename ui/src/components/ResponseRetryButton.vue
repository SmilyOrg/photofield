<template>
  <ui-button
    @click="retry"
    :disabled="loading"
    outlined
  >
    <ui-spinner
      v-if="loading"
      class="spinner"
      active
      size="small"
    >
    </ui-spinner>
    <span v-if="!loading">
      Retry
    </span>
  </ui-button>
</template>

<script setup>
import { computed } from 'vue';

const {
  response,
} = defineProps({
  response: {
    type: Object,
    required: true
  }
});

const loading = computed(() => response.isValidating.value);
const retry = () => {
  response.mutate();
};
</script>

<style scoped>
.spinner {
  vertical-align: middle;
}
</style>