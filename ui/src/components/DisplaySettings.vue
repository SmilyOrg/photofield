<template>
  <ui-card class="settings">
    <ui-select
      :modelValue="query.layout"
      @update:modelValue="emit('query', { layout: $event })"
      :options="layoutOptions"
    >
      Layout
    </ui-select>
    <div>
      <ui-icon-button
        icon="photo_size_select_small"
        :class="{ active: query.image_height == '30' }"
        @click="emit('query', { image_height: 30 })"
        outlined
      >
      </ui-icon-button>
      <ui-icon-button
        icon="photo_size_select_large"
        :class="{ active: query.image_height == '100' }"
        @click="emit('query', { image_height: query.image_height == 100 ? undefined : 100 })"
        outlined
      >
      </ui-icon-button>
      <ui-icon-button
        icon="photo_size_select_actual"
        :class="{ active: query.image_height == '300' }"
        @click="emit('query', { image_height: 300 })"
        outlined
      >
      </ui-icon-button>
      <color-mode-switch></color-mode-switch>
    </div>

    <expand-button
      :expanded="extra"
      @click="extra = !extra"
    ></expand-button>

    <div v-if="extra">
      <ui-form-field>
        <ui-checkbox
          :modelValue="query.debug_overdraw"
          @update:modelValue="emit('query', { debug_overdraw: $event })"
        ></ui-checkbox>
        <label>Debug Overdraw</label>
      </ui-form-field>
      <ui-form-field>
        <ui-checkbox
          :modelValue="query.debug_thumbnails"
          @update:modelValue="emit('query', { debug_thumbnails: $event })"
        ></ui-checkbox>
        <label>Debug Thumbnails</label>
      </ui-form-field>
    </div>

  </ui-card>
</template>

<script setup>
import { ref } from 'vue';
import ExpandButton from './ExpandButton.vue';
import ColorModeSwitch from './ColorModeSwitch.vue';

const layoutOptions = ref([
    { label: `Default`, value: "DEFAULT" },
    { label: "Album", value: "ALBUM" },
    { label: "Timeline", value: "TIMELINE" },
    { label: "Wall", value: "WALL" },
    { label: "Map", value: "MAP" },
    { label: "Highlights", value: "HIGHLIGHTS" },
    { label: "Flex", value: "FLEX" },
]);

const extra = ref(false);

const props = defineProps({
    query: Object
});

const emit = defineEmits([
    "query"
]);

</script>

<style scoped>

.settings {
  display: flex;
  align-items: center;
  border-radius: 10px;
  justify-content: center;
}

.settings > * {
  margin: 4px 10px 0 10px;
}

</style>