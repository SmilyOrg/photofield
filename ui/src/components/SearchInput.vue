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
      <ui-textfield
        v-if="active"
        ref="input"
        class="input"
        placeholder="Search your photos"
        outlined
        :modelValue="modelValue"
        @input="inputValue = $event.target.value"
        @keyup.escape="inputValue = ''; onBlur($event)"
      >
      </ui-textfield>
      <ui-textfield-helper
        v-if="active"
        class="helper"
        :visible="true"
      >
        {{ error }}
      </ui-textfield-helper>
    </div>
    <div class="chips">
      <SliderChip
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
        @change="handleSliderChange"
      />
      <DateChip
        v-model="exactDate"
        icon="event"
        placeholder="Date"
        @change="handleExactDateChange"
      />
      <DateChip
        v-if="!exactDate"
        v-model="afterDate"
        icon="event"
        placeholder="After"
        suffix=" ➔"
        @change="handleAfterDateChange"
      />
      <DateChip
        v-if="!exactDate"
        v-model="beforeDate"
        icon="event"
        placeholder="Before"
        prefix="➔ "
        @change="handleBeforeDateChange"
      />
    </div>
  </ui-form-field>
</template>

<script setup>
import { computed, ref, toRefs, watch } from 'vue';
import { watchDebounced } from '@vueuse/core'
import DateChip from './chips/DateChip.vue';
import SliderChip from './chips/SliderChip.vue';
import dateFormat from 'date-fns/format';

const props = defineProps({
  modelValue: String,
  loading: Boolean,
  error: String,
});

const {
  modelValue,
  loading,
  error,
} = toRefs(props);

const emit = defineEmits([
  "active",
  "update:modelValue",
]);

const input = ref(null);
const active = ref(true);
const inputValue = ref("");
// const afterDate = ref(null);
// const beforeDate = ref(null);

const createdAfterQualifier = {
  regex: /created:>=(\d{4}-\d{2}-\d{2})/,
  parse: (str) => new Date(str),
  replace: (date) => {
    if (!date) return '';
    return `created:>=${dateFormat(date, 'yyyy-MM-dd')}`;
  },
}

const createdBeforeQualifier = {
  regex: /created:<=(\d{4}-\d{2}-\d{2})/,
  parse: (str) => new Date(str),
  replace: (date) => {
    if (!date) return '';
    return `created:<=${dateFormat(date, 'yyyy-MM-dd')}`;
  },
}

const createdRangeQualifier = {
  regex: /created:(\d{4}-\d{2}-\d{2})..(\d{4}-\d{2}-\d{2})/,
  parse: (a, b) => [new Date(a), new Date(b)],
  replace: ([a, b]) => {
    if (!a || !b) return '';
    const astr = dateFormat(a, 'yyyy-MM-dd');
    const bstr = dateFormat(b, 'yyyy-MM-dd');
    return `created:${astr}..${bstr}`;
  },
}

const thresholdQualifier = {
  regex: /t:(\d+(\.\d+)?)/,
  parse: (str) => parseFloat(str),
  replace: (value) => {
    if (value === null || value === undefined) return '';
    return `t:${value.toFixed(3)}`;
  },
}

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
      .replace('  ', ' ')
      .trim();
  } else if (str) {
    newValue += " " + str;
  }
  inputValue.value = newValue;
  emit("update:modelValue", newValue);
}

const MIN_THRESHOLD = 0.15;
const MAX_THRESHOLD = 0.30;
const threshold = computed({
  get: () => {
    const t = extract(thresholdQualifier);
    if (t === null) return null;
    return Math.round((t - MIN_THRESHOLD) / (MAX_THRESHOLD - MIN_THRESHOLD) * 100);
  },
  set: (value) => {
    const t = value ?
      MIN_THRESHOLD + (value / 100) * (MAX_THRESHOLD - MIN_THRESHOLD)
      : null;
    inject(thresholdQualifier, t);
  }
});

const createdDateRange = computed(() => {
  return extract(createdRangeQualifier);
});

const exactDate = computed(() => {
  const range = createdDateRange.value;
  if (!range) return null;
  if (range[0].getTime() != range[1].getTime()) return null;
  return range[0];
});

const handleExactDateChange = (date) => {
  inject(createdAfterQualifier, null);
  inject(createdBeforeQualifier, null);
  inject(createdRangeQualifier, [date, date]);
};

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

const handleAfterDateChange = (date) => {
  const before = beforeDate.value;
  if (before) {
    inject(createdBeforeQualifier, date ? null : before);
    inject(createdRangeQualifier, [date, before]);
    return;
  }
  inject(createdAfterQualifier, date);
};

const handleBeforeDateChange = (date) => {
  const after = afterDate.value;
  if (after) {
    inject(createdAfterQualifier, date ? null : after);
    inject(createdRangeQualifier, [after, date]);
    return;
  }
  inject(createdBeforeQualifier, date);
};

const handleSliderChange = (value) => {
  console.log('Slider value changed:', value);
  // Add your slider logic here
};

watch(modelValue, value => {
  if (value === undefined) {
    return;
  }
  // active.value = !!value;
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
  flex-direction: column;
  align-self: baseline;
  align-items: flex-start;
}

.field.active {
  flex-grow: 1;
}

.searchbar {
  position: relative;
  display: flex;
  align-items: center;
  margin-top: -4px;
  width: 100%;
}

.chips {
  display: flex;
  gap: 8px;
  margin-top: -6px;
  flex-wrap: wrap;
}

.helper {
  position: absolute;
  left: 32px;
  bottom: 0px;
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

.helper :deep(.mdc-text-field-helper-text) {
  color: var(--mdc-theme-text-secondary-on-background) !important;
}

</style>