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
      <div
        v-if="active"
        class="input-with-helper"
      >
        <highlightable-input
          :class="{ placeholder: !leftoverText }"
          ref="input"
          :highlight="highlightRules"
          v-model="inputValue"
          @keyup.escape="inputValue = ''; onBlur($event)"
        ></highlightable-input>
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
      v-if="active && !showTextualParams"
      class="chips"
    >
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
  padding: 10px;
  box-sizing: border-box;
  min-width: 60px;
}

highlightable-input {
  max-width: 100%;
  white-space: nowrap;
  overflow-x: auto;
  padding: 4px 0;
}

highlightable-input.placeholder::after {
  content: 'dogs';
  color: var(--mdc-theme-text-secondary-on-background);
  pointer-events: none;
  opacity: 0.6;
  display: inline-block;
  animation: cycleSearchPhrases 60s infinite;
  transition: opacity 0.3s ease, transform 0.3s ease;
}

@keyframes cycleSearchPhrases {
  0%, 4%      { opacity: 0.6; transform: translateY(0);    content: 'sunset'; }
  4.5%, 5%    { opacity: 0;   transform: translateY(4px);  content: 'sunset'; }
  5%          { opacity: 0;   transform: translateY(-4px); content: 'cats'; }
  5.5%, 9%    { opacity: 0.6; transform: translateY(0);    content: 'cats'; }
  9.5%, 10%   { opacity: 0;   transform: translateY(4px);  content: 'cats'; }
  10%         { opacity: 0;   transform: translateY(-4px); content: 'night'; }
  10.5%, 14%  { opacity: 0.6; transform: translateY(0);    content: 'night'; }
  14.5%, 15%  { opacity: 0;   transform: translateY(4px);  content: 'night'; }
  15%         { opacity: 0;   transform: translateY(-4px); content: 'beach'; }
  15.5%, 19%  { opacity: 0.6; transform: translateY(0);    content: 'beach'; }
  19.5%, 20%  { opacity: 0;   transform: translateY(4px);  content: 'beach'; }
  20%         { opacity: 0;   transform: translateY(-4px); content: 'rain'; }
  20.5%, 24%  { opacity: 0.6; transform: translateY(0);    content: 'rain'; }
  24.5%, 25%  { opacity: 0;   transform: translateY(4px);  content: 'rain'; }
  25%         { opacity: 0;   transform: translateY(-4px); content: 'snow'; }
  25.5%, 29%  { opacity: 0.6; transform: translateY(0);    content: 'snow'; }
  29.5%, 30%  { opacity: 0;   transform: translateY(4px);  content: 'snow'; }
  30%         { opacity: 0;   transform: translateY(-4px); content: 'forest'; }
  30.5%, 34%  { opacity: 0.6; transform: translateY(0);    content: 'forest'; }
  34.5%, 35%  { opacity: 0;   transform: translateY(4px);  content: 'forest'; }
  35%         { opacity: 0;   transform: translateY(-4px); content: 'city'; }
  35.5%, 39%  { opacity: 0.6; transform: translateY(0);    content: 'city'; }
  39.5%, 40%  { opacity: 0;   transform: translateY(4px);  content: 'city'; }
  40%         { opacity: 0;   transform: translateY(-4px); content: 'food'; }
  40.5%, 44%  { opacity: 0.6; transform: translateY(0);    content: 'food'; }
  44.5%, 45%  { opacity: 0;   transform: translateY(4px);  content: 'food'; }
  45%         { opacity: 0;   transform: translateY(-4px); content: 'car'; }
  45.5%, 49%  { opacity: 0.6; transform: translateY(0);    content: 'car'; }
  49.5%, 50%  { opacity: 0;   transform: translateY(4px);  content: 'car'; }
  50%         { opacity: 0;   transform: translateY(-4px); content: 'dogs'; }
  50.5%, 54%  { opacity: 0.6; transform: translateY(0);    content: 'dogs'; }
  54.5%, 55%  { opacity: 0;   transform: translateY(4px);  content: 'dogs'; }
  55%         { opacity: 0;   transform: translateY(-4px); content: 'bird'; }
  55.5%, 59%  { opacity: 0.6; transform: translateY(0);    content: 'bird'; }
  59.5%, 60%  { opacity: 0;   transform: translateY(4px);  content: 'bird'; }
  60%         { opacity: 0;   transform: translateY(-4px); content: 'sky'; }
  60.5%, 64%  { opacity: 0.6; transform: translateY(0);    content: 'sky'; }
  64.5%, 65%  { opacity: 0;   transform: translateY(4px);  content: 'sky'; }
  65%         { opacity: 0;   transform: translateY(-4px); content: 'water'; }
  65.5%, 69%  { opacity: 0.6; transform: translateY(0);    content: 'water'; }
  69.5%, 70%  { opacity: 0;   transform: translateY(4px);  content: 'water'; }
  70%         { opacity: 0;   transform: translateY(-4px); content: 'party'; }
  70.5%, 74%  { opacity: 0.6; transform: translateY(0);    content: 'party'; }
  74.5%, 75%  { opacity: 0;   transform: translateY(4px);  content: 'party'; }
  75%         { opacity: 0;   transform: translateY(-4px); content: 'happy'; }
  75.5%, 79%  { opacity: 0.6; transform: translateY(0);    content: 'happy'; }
  79.5%, 80%  { opacity: 0;   transform: translateY(4px);  content: 'happy'; }
  80%         { opacity: 0;   transform: translateY(-4px); content: 'tree'; }
  80.5%, 84%  { opacity: 0.6; transform: translateY(0);    content: 'tree'; }
  84.5%, 85%  { opacity: 0;   transform: translateY(4px);  content: 'tree'; }
  85%         { opacity: 0;   transform: translateY(-4px); content: 'smile'; }
  85.5%, 89%  { opacity: 0.6; transform: translateY(0);    content: 'smile'; }
  89.5%, 90%  { opacity: 0;   transform: translateY(4px);  content: 'smile'; }
  90%         { opacity: 0;   transform: translateY(-4px); content: 'flower'; }
  90.5%, 94%  { opacity: 0.6; transform: translateY(0);    content: 'flower'; }
  94.5%, 95%  { opacity: 0;   transform: translateY(4px);  content: 'flower'; }
  95%         { opacity: 0;   transform: translateY(-4px); content: 'art'; }
  95.5%, 99%  { opacity: 0.6; transform: translateY(0);    content: 'art'; }
  99.5%, 100% { opacity: 0;   transform: translateY(4px);  content: 'art'; }
}

highlightable-input :deep(mark) {
  white-space: pre;
  background: none;
  color: var(--mdc-theme-text-secondary-on-background);
  transform: translateX(4px);
  transition: opacity 0.3s ease, transform 0.3s ease;
}

.field.show-textual highlightable-input :deep(mark) {
  display: inline-block;
  opacity: 1;
  transform: translateX(0);
}

.chips {
  display: flex;
  gap: 8px;
  overflow-x: scroll;
  height: fit-content;
  padding: 8px;
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