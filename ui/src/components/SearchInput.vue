<template>
  <ui-form-field
    class="field"
    :class="{ active: active, 'show-textual': showTextualParams }"
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
      <!-- <ui-textfield
        v-if="active"
        ref="input"
        class="input"
        placeholder="Search your photos"
        outlined
        :modelValue="modelValue"
        @input="inputValue = $event.target.value"
        @keyup.escape="inputValue = ''; onBlur($event)"
      >
      </ui-textfield> -->
      <highlightable-input
        v-if="active"
        ref="input"
        placeholder="Search your photos"
        :highlight="highlightRules"
        v-model="inputValue"
        @keyup.escape="inputValue = ''; onBlur($event)"
      ></highlightable-input>
      <ui-textfield-helper
        v-if="active"
        class="helper"
        :visible="true"
      >
        {{ error }}
      </ui-textfield-helper>
    </div>
    <div v-if="!showTextualParams" class="chips">
      <SliderChip
        v-if="leftoverText.length > 0"
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
        icon="event"
        placeholder="Date"
        @change="handleExactDateChange"
      />
      <DateChip
        v-if="!exactDate"
        :model-value="afterDate"
        icon="event"
        placeholder="After"
        suffix=" ➔"
        @change="handleAfterDateChange"
      />
      <DateChip
        v-if="!exactDate"
        :model-value="beforeDate"
        icon="event"
        placeholder="Before"
        prefix="➔ "
        @change="handleBeforeDateChange"
      />
    </div>
    <!-- <ui-icon-button
      :aria-label="showTextualParams ? 'Show chips' : 'Show text params'"
      class="toggle-params-button"
      @click="showTextualParams = !showTextualParams"
    >
      {{ showTextualParams ? 'grid_view' : 'code' }}
    </ui-icon-button> -->
  </ui-form-field>
</template>

<script setup>
import { computed, nextTick, ref, shallowRef, toRefs, watch } from 'vue';
import { watchDebounced } from '@vueuse/core'
import DateChip from './chips/DateChip.vue';
import SliderChip from './chips/SliderChip.vue';
import dateFormat from 'date-fns/format';
import HighlightableInput from 'highlightable-input/vue'

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

const input = shallowRef();
const active = ref(false);
const inputValue = ref("");
const showTextualParams = ref(false);

const toggle = async () => {
  active.value = !active.value;
  if (active.value) {
    await nextTick();
    input.value?.$el?.focus();
  }
}

const onBlur = () => {
  if (!inputValue.value) {
    active.value = false;
  }
}

const createdAfterQualifier = {
  name: "createdAfter",
  regex: / created:>=(\d{4}-\d{2}-\d{2})/,
  parse: (str) => new Date(str),
  replace: (date) => {
    if (!date) return '';
    return ` created:>=${dateFormat(date, 'yyyy-MM-dd')}`;
  },
}

const createdBeforeQualifier = {
  name: "createdBefore",
  regex: / created:<=(\d{4}-\d{2}-\d{2})/,
  parse: (str) => new Date(str),
  replace: (date) => {
    if (!date) return '';
    return ` created:<=${dateFormat(date, 'yyyy-MM-dd')}`;
  },
}

const createdRangeQualifier = {
  name: "createdRange",
  regex: / created:(\d{4}-\d{2}-\d{2})..(\d{4}-\d{2}-\d{2})/,
  parse: (a, b) => [new Date(a), new Date(b)],
  replace: ([a, b]) => {
    if (!a || !b) return '';
    const astr = dateFormat(a, 'yyyy-MM-dd');
    const bstr = dateFormat(b, 'yyyy-MM-dd');
    return ` created:${astr}..${bstr}`;
  },
}

const thresholdQualifier = {
  name: "threshold",
  regex: / t:(\d+(\.\d+)?)/,
  parse: (str) => parseFloat(str),
  replace: (value) => {
    if (value === null || value === undefined) return '';
    return ` t:${value.toFixed(3)}`;
  },
}

const qualifiers = [
  createdAfterQualifier,
  createdBeforeQualifier,
  createdRangeQualifier,
  thresholdQualifier,
];

const highlightRules = computed(() => {
  return qualifiers.map(q => ({
    pattern: q.regex,
    class: q.name,
  }));
});

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
      .replace('  ', ' ');
  } else if (str) {
    newValue += str + " ";
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

const leftoverText = computed(() => {
  let text = inputValue.value;
  qualifiers.forEach(q => {
    text = text.replace(q.regex, '');
  });
  return text.trim();
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
  /* align-self: baseline; */
  align-items: center;
  overflow-x: scroll;
  height: fit-content;
  /* width: 100%; */
}

.field.active {
  flex-grow: 1;
}

.searchbar {
  position: relative;
  display: flex;
  align-items: center;
  margin-right: 16px;
  /* width: 100%; */
}

.field.active .searchbar {
  /* margin-top: -4px; */
}

highlightable-input {
  white-space: nowrap;
}

highlightable-input :deep(mark) {
  white-space: pre;
  display: none;
  background: none;
  color: var(--mdc-theme-text-secondary-on-background);
  opacity: 0;
  transform: translateX(4px);
  transition: opacity 0.3s ease, transform 0.3s ease;
}

.field.show-textual highlightable-input :deep(mark) {
  display: inline-block;
  opacity: 1;
  transform: translateX(0);
}

highlightable-input:focus-within :deep(mark),
highlightable-input :deep(mark:has(+ *:focus)),
highlightable-input :deep(mark:focus) {
  /* position: unset; */
  /* opacity: 1; */
  /* left: 0; */
  /* transform: translateX(0); */
  /* transition: ; */
}

.chips {
  display: flex;
  gap: 8px;
  /* margin-top: -6px; */
  /* flex-wrap: wrap; */
  overflow-x: scroll;
  height: fit-content;
}

.toggle-params-button {
  flex-shrink: 0;
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