<template>
  <ui-card class="collection-panel">
    <collection-settings
      :collection="collection"
      :scenes="scenes"
      :tasks="tasks"
      @reindex="emit('reindex')"
      @expand="(expand = $event)"
    >
    </collection-settings>
        
    <ui-divider></ui-divider>
      
    <div class="scrollable">
      <collection-debug
        v-if="expand"
        :collection="collection"
        :scenes="scenes"
        :tasks="tasks"
        @reload="emit('reload', $event)"
      >
      </collection-debug>

      <ui-list
        class="list"
        v-if="collections?.length > 0"
      >
          <router-link
            v-for="c in collections"
            :key="c.id"
            class="link"
            :to="'/collections/' + c.id"
            @click="emit('close')"
          >
            <ui-item
              :active="c.id == collection?.id"
            >
                {{ c.name }}
            </ui-item>
          </router-link>
      </ui-list>
    </div>
  </ui-card>
</template>

<script setup>
import CollectionSettings from './CollectionSettings.vue';
import CollectionDebug from './CollectionDebug.vue';
import { ref } from 'vue';

const props = defineProps({
    collections: Array,
    collection: Object,
    scenes: Array,
    tasks: Array,
});

const emit = defineEmits([
    "close",
    "reindex",
    "reload",
]);

const expand = ref(false);

</script>

<style scoped>

.collection-panel {
  max-width: 600px;
  background: var(--mdc-theme-background);
  border-radius: 10px;
  padding: 0 16px 16px 16px;
  display: flex;
  flex-direction: column;
}

.link {
  text-decoration: none;
  color: var(--mdc-theme-text-primary-on-background);
}

.tight {
  margin-bottom: 0;
}

.list {
  height: 100%;
}

.scrollable {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  max-height: 600px;
  overflow-y: auto;
}

</style>