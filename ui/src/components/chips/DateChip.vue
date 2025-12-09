<template>
  <chip
    :icon="icon"
    :text="displayText"
    clickable
    :selected="modelValue !== null"
    :removable="modelValue !== null"
    @click="openDatePicker"
    @remove="clear"
  />
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue';
import flatpickr from 'flatpickr';
import 'flatpickr/dist/flatpickr.css';
import Chip from './Chip.vue';
import dateFormat from 'date-fns/format';

const props = defineProps({
  modelValue: {
    type: [Date, String, Number],
    default: null,
  },
  icon: {
    type: String,
    default: 'event',
  },
  placeholder: {
    type: String,
    default: 'Select date',
  },
  mode: {
    type: String,
    default: 'single', // 'single', 'multiple', 'range'
  },
  dateFormat: {
    type: String,
    default: 'Y-m-d',
  },
  enableTime: {
    type: Boolean,
    default: false,
  },
  options: {
    type: Object,
    default: () => ({}),
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

const emit = defineEmits([
  'update:modelValue',
  'change',
]);

const flatpickrInstance = ref(null);

const displayText = computed(() => {
  if (!props.modelValue) {
    return props.placeholder;
  }
  
  if (props.mode === 'range' && Array.isArray(props.modelValue)) {
    return props.modelValue.map(d => formatDate(d)).join(' - ');
  }
  
  if (Array.isArray(props.modelValue)) {
    return `${props.modelValue.length} date(s)`;
  }

  return props.prefix + formatDate(props.modelValue) + props.suffix;
});

const formatDate = (date) => {
  return dateFormat(date, "d MMM yyyy");
};

const openDatePicker = (event) => {
  if (!flatpickrInstance.value) {
    initializeFlatpickr(event.target);
  }
  flatpickrInstance.value.open();
};

const clear = () => {
  emit('update:modelValue', null);
  emit('change', null);
};

const initializeFlatpickr = (el) => {
  const flatpickrOptions = {
    mode: props.mode,
    dateFormat: props.dateFormat,
    enableTime: props.enableTime,
    defaultDate: props.modelValue,
    onChange: (selectedDates, dateStr, instance) => {
      const value = props.mode === 'single' ? selectedDates[0] : selectedDates;
      emit('update:modelValue', value);
      emit('change', value);
    },
    onClose: async () => {
      await nextTick();
      if (flatpickrInstance.value) {
        flatpickrInstance.value.destroy();
        flatpickrInstance.value = null;
      }
    },
    ...props.options,
  };
  flatpickrInstance.value = flatpickr(el, flatpickrOptions);
};

onBeforeUnmount(() => {
  if (flatpickrInstance.value) {
    flatpickrInstance.value.destroy();
    flatpickrInstance.value = null;
  }
});
</script>

<style scoped>
/* Flatpickr will handle its own styling */
</style>
