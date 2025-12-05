<template>
  <div class="slider-chip" ref="sliderChip" @mousedown="handleMouseDown">
    <chip
      :icon="icon"
      :text="displayText"
      clickable
      :selected="modelValue !== null"
      :removable="modelValue !== null"
      @click="handleChipClick"
      @remove="clear"
    />
    <div
      v-if="showSlider"
      class="slider-container"
      ref="sliderContainer"
      @focusout="handleFocusOut"
    >
      <ui-slider
        ref="slider"
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
import { ref, computed, watch, nextTick } from 'vue';
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

const showSlider = ref(false);
const sliderContainer = ref(null);
const slider = ref(null);
const sliderValue = ref(props.modelValue ?? props.defaultValue);
const sliderChip = ref(null);
const isMouseDownInComponent = ref(false);

const displayText = computed(() => {
  if (props.modelValue === null) {
    return props.placeholder;
  }
  return props.prefix + props.formatValue(props.modelValue) + props.suffix;
});

const handleMouseDown = () => {
  isMouseDownInComponent.value = true;
  // Reset on next tick to allow click to happen
  setTimeout(() => {
    isMouseDownInComponent.value = false;
  }, 0);
};

const handleChipClick = async () => {
  // If disabled (null), enable it with default value
  if (props.modelValue === null) {
    sliderValue.value = props.defaultValue;
    emit('update:modelValue', props.defaultValue);
    emit('change', props.defaultValue);
  }

  // If slider is already open, just refocus it instead of toggling
  if (showSlider.value) {
    showSlider.value = false
    return;
  }

  // Open the slider
  showSlider.value = true;
  
  // Wait for DOM update and focus the slider input
  await nextTick();
  if (slider.value?.$el) {
    // Find the actual input element within the ui-slider component
    const input = slider.value.$el.querySelector('input');
    if (input) {
      input.focus();
    }
  }
};

const handleFocusOut = (event) => {
  // If mouse is down within the component, don't close the slider
  // This prevents closing when clicking the chip while slider is open
  if (isMouseDownInComponent.value) {
    return;
  }
  
  // focusout bubbles, so we can catch it from child elements
  // Check if the new focus target is outside the entire slider-chip component
  if (!sliderChip.value?.contains(event.relatedTarget)) {
    showSlider.value = false;
  }
};

const clear = () => {
  emit('update:modelValue', null);
  emit('change', null);
  showSlider.value = false;
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
  outline: none;
}
</style>
