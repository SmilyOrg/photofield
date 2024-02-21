<template>
  <ui-form-field
    class="field"
    :class="{ active: active }"
  >
    <ui-icon-button
      :class="{ hidden: loading }"
      @click="active = !active"
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

<script>
import { nextTick, ref, toRef, watch } from 'vue';
import { watchDebounced } from '@vueuse/core'

export default {
  emits: ["active"],
  props: ["modelValue", "loading", "error"],
  setup(props, { emit }) {
    const modelValue = toRef(props, "modelValue");

    const input = ref(null);
    const active = ref(false);
    const inputValue = ref("");
    
    const onBlur = () => {
      if (inputValue.value == "") {
        active.value = false;
      }
    }

    watch(modelValue, value => {
      if (value) {
        active.value = true;
        inputValue.value = value;
      } else {
        active.value = false;
      }
    }, {
      immediate: true,
    })

    watch(active, async value => {
      emit("active", value);
      if (value) {
        await nextTick();
        const inputEl = input.value.textfield.querySelector("input");
        inputEl.focus()
      } else {
        inputValue.value = "";
      }
    }, {
      immediate: true,
    });

    watchDebounced(
      inputValue,
      newValue => { emit("update:modelValue", newValue); },
      { debounce: 500 },
    );

    return {
      input,
      active,
      onBlur,
      inputValue,
    }
  }
};
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

</style>