<template>
  <ui-form-field
    class="field"
    :class="{ active: active }"
  >
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
      @blur="onBlur"
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
  </ui-form-field>
</template>

<script setup>
import { nextTick, ref, toRefs, watch } from 'vue';
import { watchDebounced } from '@vueuse/core'

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
const active = ref(false);
const inputValue = ref("");

const onBlur = () => {
  if (!inputValue.value) {
    active.value = false;
  }
}

const toggle = async () => {
  active.value = !active.value;
  if (active.value) {
    await nextTick();
    const inputEl = input.value.textfield.querySelector("input");
    inputEl.focus()
  }
}

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
}

.helper {
  position: absolute;
  left: 32px;
  bottom: 0px;
}

.field.active {
  flex-grow: 1;
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