<template>
  <ui-card class="settings">
    <ui-select
      :modelValue="layoutValue"
      @update:modelValue="onLayoutChange"
      :options="layoutOptions"
    >
      Layout
    </ui-select>
    <ui-select
      :modelValue="sortValue"
      @update:modelValue="onSortChange"
      :options="sortOptions"
    >
      Sort
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
import { ref, computed } from 'vue';
import ExpandButton from './ExpandButton.vue';
import ColorModeSwitch from './ColorModeSwitch.vue';

const extra = ref(false);

const props = defineProps({
    query: Object,
    collection: Object
});

const emit = defineEmits([
    "query"
]);

// Determine the default layout from collection config
const defaultLayout = computed(() => {
    return props.collection?.layout || "ALBUM";
});

const layoutOptions = computed(() => {
    const def = defaultLayout.value;
    
    const options = [
        { label: "Album", value: "ALBUM" },
        { label: "Timeline", value: "TIMELINE" },
        { label: "Wall", value: "WALL" },
        { label: "Map", value: "MAP" },
        { label: "Highlights", value: "HIGHLIGHTS" },
        { label: "Flex", value: "FLEX" },
    ];
    
    const defaultOption = options.find(opt => opt.value === def);
    const defaultLabel = defaultOption ? defaultOption.label : "Default";
    
    return [
        { label: `${defaultLabel}*`, value: "DEFAULT" },
        ...options,
    ];
});

const layoutValue = computed(() => {
    const layout = props.query?.layout;
    if (!layout || layout === 'DEFAULT') {
        return "DEFAULT";
    }
    return layout;
});

const onLayoutChange = (value) => {
    if (!value || value === 'DEFAULT') {
        emit('query', { layout: undefined });
    } else {
        emit('query', { layout: value });
    }
};

// Determine the default sort based on collection config, then layout
const defaultSort = computed(() => {
    // Use collection's configured sort if available
    if (props.collection?.sort) {
        return props.collection.sort;
    }
    // Fall back to layout-based defaults
    const layout = props.query?.layout;
    if (layout === 'TIMELINE') {
        return '-date'; // Newest first for timeline
    }
    return '+date'; // Oldest first for others
});

const sortOptions = computed(() => {
    const def = defaultSort.value;
    
    const options = [
        { label: "Oldest First", value: '+date' },
        { label: "Newest First", value: '-date' },
        { label: "Shuffle (Hourly)", value: "+shuffle-hourly" },
        { label: "Shuffle (Daily)", value: "+shuffle-daily" },
        { label: "Shuffle (Weekly)", value: "+shuffle-weekly" },
        { label: "Shuffle (Monthly)", value: "+shuffle-monthly" },
    ];
    
    const defaultOption = options.find(opt => opt.value === def);
    const defaultLabel = defaultOption ? defaultOption.label : "Default";
    
    return [
        { label: `${defaultLabel}*`, value: "DEFAULT" },
        ...options,
    ];
});

const sortValue = computed(() => {
    const sort = props.query?.sort;
    if (!sort) {
        return "DEFAULT";
    }
    return sort;
});

const onSortChange = (value) => {
    if (!value || value === 'DEFAULT') {
        // Clear sort to use default
        const updates = { sort: undefined };
        
        // No need to change layout when going back to default
        emit('query', updates);
    } else if (value.startsWith('+shuffle-')) {
        // If shuffle is selected and layout is DEFAULT, switch to FLEX
        const updates = { sort: value };
        if (!props.query?.layout || props.query.layout === 'DEFAULT') {
            updates.layout = 'FLEX';
        }
        emit('query', updates);
    } else {
        // Regular sort (date ascending/descending)
        emit('query', { sort: value });
    }
};

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