<template>
  <div class="container">
    <page-title title="Photos"></page-title>

    <!-- <response :response="response"></response> -->
    <response-loader
      :response="response"
    ></response-loader>
    <center-message
      icon="image_not_supported"
      v-if="collections?.length === 0"
    >
      <h1>No collections found!</h1>
      This may be because:
      <ul>
        <li>You are using the default configuration and there are no folders in the working directory.</li>
        <li>You have not configured any collections in the <code>configuration.yaml</code>.</li>
        <li>Your <code>configuration.yaml</code> is not being loaded correctly.</li>
      </ul>
      <response-retry-button @click="reload" :response="response">
        Reload Configuration
      </response-retry-button>
    </center-message>
    <div
      v-else
      class="collections"
    >
      <collection-link
        class="collection"
        v-for="c in collections"
        :key="c.id"
        :collection="c"
      ></collection-link>
    </div>
  </div>
</template>

<script setup>
import { createTask, useApi } from '../api';
import PageTitle from './PageTitle.vue';
import CollectionLink from './CollectionLink.vue';
import ResponseLoader from './ResponseLoader.vue';
import ResponseRetryButton from './ResponseRetryButton.vue';
import CenterMessage from './CenterMessage.vue';

const response = useApi(() => "/collections");
const collections = response.items;

const reload = async () => {
  await createTask("RELOAD_CONFIG");
  await response.mutate();
}
</script>

<style scoped>

.collections {
  margin: 20px;
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
}

.config {
  max-height: 400px;
  overflow: auto;
}

</style>
