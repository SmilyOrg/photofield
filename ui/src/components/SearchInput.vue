<template>
  <ui-form-field
    class="field"
    :class="{ active: active }"
  >
    <div class="searchbar">
      <ui-icon-button
        :aria-label="active ? 'Close search' : 'Open search'"
        :class="{ hidden: loading }"
        @click="toggle()"
      >
        {{ active ? "close" : "search" }}
      </ui-icon-button>
      <ui-spinner
        v-if="active && loading"
        class="spinner"
        active
        size="small"
      ></ui-spinner>
      <div
        v-if="active"
        class="input-with-helper"
      >
        <highlighted-input
          v-if="active"
          ref="input"
          class="input"
          placeholder="Search your photos"
          outlined
          v-model="inputValue"
          :tokens="tokens"
          @keyup.escape="inputValue = ''; active = false"
        />
        <ui-textfield-helper
          v-if="error"
          class="helper"
          :visible="true"
        >
          {{ error }}
        </ui-textfield-helper>
      </div>
    </div>
    <div
      v-if="active"
      class="chips"
    >
      <ImageChip
        v-if="imageId !== null"
        v-model="imageId"
        :scene="scene"
        placeholder="Image"
      />
      <SliderChip
        v-if="leftoverText.length > 0 || imageId !== null"
        v-model="threshold"
        icon="tune"
        placeholder="Filter"
        :format-value="value => value.toFixed(0)"
        :default-value="75"
        :min="0"
        :max="100"
        :step="1"
        prefix="Filter: "
        suffix="%"
      />
      <DateChip
        :model-value="exactDate"
        :default-value="beforeDate || afterDate || firstTimestamp"
        icon="event"
        placeholder="Date"
        @change="handleExactDateChange"
      />
      <DateChip
        :model-value="afterDate"
        :default-value="beforeDate || exactDate || firstTimestamp"
        icon="event"
        placeholder="After"
        suffix=" ➔"
        @change="handleAfterDateChange"
      />
      <DateChip
        :model-value="beforeDate"
        :default-value="afterDate || exactDate || lastTimestamp"
        icon="event"
        placeholder="Before"
        prefix="➔ "
        @change="handleBeforeDateChange"
      />
    </div>
  </ui-form-field>
</template>

<script setup>
import { computed, nextTick, ref, shallowRef, toRefs, watch } from 'vue';
import { watchDebounced } from '@vueuse/core'
import DateChip from './chips/DateChip.vue';
import SliderChip from './chips/SliderChip.vue';
import ImageChip from './chips/ImageChip.vue';
import dateFormat from 'date-fns/format';
import parseISO from 'date-fns/parseISO';
import HighlightedInput from './HighlightedInput.vue';
import { useApi } from '../api';
import { useTimestamps, useTimestampsDate } from '../use';

const props = defineProps({
  modelValue: String,
  scene: Object,
  viewport: Object,
  loading: Boolean,
  error: String,
});

const {
  modelValue,
  scene,
  viewport,
  loading,
  error,
} = toRefs(props);

const emit = defineEmits([
  "active",
  "update:modelValue",
]);

const input = shallowRef();
const active = ref(false);
const inputValue = ref("");

const { items: searchQueries } = useApi(() => {
  const q = inputValue.value;
  const sceneId = scene.value?.id;
  return q && sceneId && `/scenes/${sceneId}/search-queries?search=${encodeURIComponent(q)}`;
});

const viewportHeight = computed(() => viewport.value?.height);
const timestamps = useTimestamps({ scene, height: viewportHeight });
const firstTimestamp = useTimestampsDate({ timestamps, ratio: ref(0) })
const lastTimestamp = useTimestampsDate({ timestamps, ratio: ref(1) })

const query = computed(() => {
  const items = searchQueries.value;
  return items && items.length > 0 ? items[0] : null;
});

const tokens = computed(() => {
  return query.value?.tokens || [];
});


const toggle = async () => {
  active.value = !active.value;
  if (active.value) {
    await nextTick();
    input.value?.focus();
  }
}
const createdAfterQualifier = {
  name: "createdAfter",
  regex: /created:>=(\d{4}-\d{2}-\d{2})/,
  parse: (str) => parseISO(str),
  replace: (date) => {
    if (!date) return '';
    return `created:>=${dateFormat(date, 'yyyy-MM-dd')}`;
  },
}
const createdBeforeQualifier = {
  name: "createdBefore",
  regex: /created:<=(\d{4}-\d{2}-\d{2})/,
  parse: (str) => parseISO(str),
  replace: (date) => {
    if (!date) return '';
    return `created:<=${dateFormat(date, 'yyyy-MM-dd')}`;
  },
}
const createdExactQualifier = {
  name: "createdExact",
  regex: /created:(\d{4}-\d{2}-\d{2})(?!\.\.)/,
  parse: (str) => parseISO(str),
  replace: (date) => {
    if (!date) return '';
    return `created:${dateFormat(date, 'yyyy-MM-dd')}`;
  },
}
const createdRangeQualifier = {
  name: "createdRange",
  regex: /created:(\d{4}-\d{2}-\d{2})\.\.(\d{4}-\d{2}-\d{2})/,
  parse: (a, b) => [parseISO(a), parseISO(b)],
  replace: (range) => {
    if (!range || range.length !== 2) return '';
    const [a, b] = range;
    if (!a || !b) return '';
    const astr = dateFormat(a, 'yyyy-MM-dd');
    const bstr = dateFormat(b, 'yyyy-MM-dd');
    return `created:${astr}..${bstr}`;
  },
}

const thresholdQualifier = {
  name: "threshold",
  regex: /t:(\d+(\.\d+)?)/,
  parse: (str) => parseFloat(str),
  replace: (value) => {
    if (value === null || value === undefined) return '';
    return `t:${value.toFixed(3)}`;
  },
}

const imageIdQualifier = {
  name: "imageId",
  regex: /img:(\d+)/,
  parse: (str) => str,
  replace: (value) => {
    if (!value) return '';
    return `img:${value}`;
  },
}

const qualifiers = [
  createdAfterQualifier,
  createdBeforeQualifier,
  createdRangeQualifier,
  createdExactQualifier,
  thresholdQualifier,
  imageIdQualifier,
];

const extract = (qualifier) => {
  const input = modelValue.value;
  if (!input) return null;
  const match = input.match(qualifier.regex);
  if (match) {
    return qualifier.parse(...match.slice(1));
  }
  return null;
}

const inject = (qualifier, value) => {
  const str = qualifier.replace ? qualifier.replace(value) : value;
  let newValue = inputValue.value;
  if (newValue.match(qualifier.regex)) {
    newValue = inputValue.value
      .replace(qualifier.regex, str)
      .replace(/ +/g, ' ');
  } else if (str) {
    newValue = str + ' ' + newValue.trim();
  }
  inputValue.value = newValue;
  emit("update:modelValue", newValue);
}

const thresholdRange = computed(() => {
  if (imageId.value !== null) {
    return [0.500, 0.999];
  }
  return [0.15, 0.30];
});
const threshold = computed({
  get: () => {
    const t = extract(thresholdQualifier);
    if (t === null) return null;
    return Math.round((t - thresholdRange.value[0]) / (thresholdRange.value[1] - thresholdRange.value[0]) * 100);
  },
  set: (value) => {
    const t = value !== null ?
      thresholdRange.value[0] + (value / 100) * (thresholdRange.value[1] - thresholdRange.value[0])
      : null;
    inject(thresholdQualifier, t);
  }
});

const exactDate = computed(() => {
  return extract(createdExactQualifier);
});

const createdDateRange = computed(() => {
  return extract(createdRangeQualifier);
});


const afterDate = computed(() => {
  const range = createdDateRange.value;
  if (range) {
    return range[0];
  }
  return extract(createdAfterQualifier);
});

const beforeDate = computed(() => {
  const range = createdDateRange.value;
  if (range) {
    return range[1];
  }
  return extract(createdBeforeQualifier);
});

const imageId = computed({
  get: () => extract(imageIdQualifier),
  set: (value) => inject(imageIdQualifier, value),
});

const leftoverText = computed(() => {
  let text = inputValue.value;
  qualifiers.forEach(q => {
    text = text.replace(q.regex, '');
  });
  return text.trim();
});

const handleAfterDateChange = (date) => {
  inject(createdExactQualifier, null);
  const before = beforeDate.value;
  if (before) {
    inject(createdBeforeQualifier, date ? null : before);
    inject(createdRangeQualifier, [date, before]);
    return;
  }
  inject(createdAfterQualifier, date);
};

const handleBeforeDateChange = (date) => {
  inject(createdExactQualifier, null);
  const after = afterDate.value;
  if (after) {
    inject(createdAfterQualifier, date ? null : after);
    inject(createdRangeQualifier, [after, date]);
    return;
  }
  inject(createdBeforeQualifier, date);
};

const handleExactDateChange = (date) => {
  inject(createdAfterQualifier, null);
  inject(createdBeforeQualifier, null);
  inject(createdRangeQualifier, null);
  inject(createdExactQualifier, date);
};


watch(modelValue, value => {
  if (value === undefined) {
    return;
  }
  active.value = !!value;
  inputValue.value = value;
}, {
  immediate: true,
})

watch(active, async value => {
  emit("active", value);
  if (!value) {
    inputValue.value = "";
  }
}, {
  immediate: true,
});

watchDebounced(
  inputValue,
  newValue => {
    emit("update:modelValue", newValue);
  },
  { debounce: 1000 },
);
</script>

<style scoped>

.field {
  position: relative;
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  align-items: start;
  overflow-x: scroll;
  height: fit-content;
}

.field.active {
  flex-grow: 1;
}

.searchbar {
  position: relative;
  display: flex;
  align-items: start;
  max-width: 100%;
  flex-grow: 1;
}

.input-with-helper {
  display: flex;
  flex-direction: column;
  flex-grow: 1;
  box-sizing: border-box;
  min-width: 60px;
}

.helper :deep(.mdc-text-field-helper-text) {
  margin: -14px 8px 8px 0;
  color: var(--mdc-theme-error) !important;
  background-color: var(--mdc-theme-background);
  outline: 8px solid var(--mdc-theme-background);
  border-radius: 8px;
}

.chips {
  display: flex;
  gap: 8px;
  overflow-x: scroll;
  height: fit-content;
  padding: 8px;
}

.chips > * {
  animation: slideInFade 0.3s ease-out forwards;
  opacity: 0;
  transform: translateY(-8px);
}

.chips > *:nth-child(1) {
  animation-delay: 150ms;
}

.chips > *:nth-child(2) {
  animation-delay: 180ms;
}

.chips > *:nth-child(3) {
  animation-delay: 210ms;
}

.chips > *:nth-child(4) {
  animation-delay: 240ms;
}

@keyframes slideInFade {
  from {
    opacity: 0;
    transform: translateY(-8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);   
  } 
}

.input {
  padding-left: 0;
  flex-grow: 1;
}

.input :deep(.mdc-notched-outline) {
  display: none;
}

.hidden {
  opacity: 0;
}

.spinner {
  position: absolute;
  top: 16px;
  left: 12px;
}

</style>