<template>
  <div class="slider-chip">
    <chip
      :icon="icon"
      :text="displayText"
      clickable
      :selected="modelValue !== null"
      :removable="modelValue !== null"
      @click="toggleExpanded"
      @remove="clear"
    />
    <div v-if="expanded" class="slider-container">
      <ui-slider
        v-model="sliderValue"
        :min="min"
        :max="max"
        :step="step"
        :discrete="discrete"
        :markers="markers"
        @update:modelValue="handleInput"
      ></ui-slider>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue';
import Chip from './Chip.vue';

const props = defineProps({
  modelValue: {
    type: Number,
    default: null,
  },
  icon: {
    type: String,
    default: 'tune',
  },
  placeholder: {
    type: String,
    default: 'Slider',
  },
  min: {
    type: Number,
    default: 0,
  },
  max: {
    type: Number,
    default: 100,
  },
  step: {
    type: Number,
    default: 1,
  },
  discrete: {
    type: Boolean,
    default: true,
  },
  markers: {
    type: Boolean,
    default: false,
  },
  defaultValue: {
    type: Number,
    default: 0,
  },
  formatValue: {
    type: Function,
    default: (value) => value,
  },
  prefix: {
    type: String,
    default: '',
  },
  suffix: {
    type: String,
    default: '',
  },
});

const emit = defineEmits(['update:modelValue', 'change']);

const expanded = ref(false);
const sliderValue = ref(props.modelValue ?? props.defaultValue);

const displayText = computed(() => {
  if (props.modelValue === null) {
    return props.placeholder;
  }
  return props.prefix + props.formatValue(props.modelValue) + props.suffix;
});

const toggleExpanded = () => {
  // If disabled (null), enable it with default value
  if (props.modelValue === null) {
    sliderValue.value = props.defaultValue;
    emit('update:modelValue', props.defaultValue);
    emit('change', props.defaultValue);
  }
  expanded.value = !expanded.value;
};

const clear = () => {
  emit('update:modelValue', null);
  emit('change', null);
  expanded.value = false;
};

const handleInput = (event) => {
  emit('update:modelValue', event);
  emit('change', event);
};

watch(() => props.modelValue, (newValue) => {
  if (newValue !== null && newValue !== undefined) {
    sliderValue.value = newValue;
  }
});
</script>

<style scoped>
.slider-chip {
  display: inline-flex;
  flex-direction: column;
  gap: 8px;
}

.slider-container {
  background-color: var(--mdc-theme-surface, #e0e0e0);
  border-radius: 16px;
  min-width: 200px;
}
</style>
