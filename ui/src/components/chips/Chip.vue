<template>
  <button
    class="chip"
    :class="{
      'chip--clickable': clickable,
      'chip--selected': selected,
      'chip--outlined': outlined,
      'chip--dense': dense,
      'chip--disabled': disabled,
    }"
    :tabindex="clickable && !disabled ? 0 : -1"
    @click="handleClick"
    @keydown.enter="handleClick"
    @keydown.space.prevent="handleClick"
  >
    <ui-icon
      v-if="icon"
      class="chip__icon"
      :size="iconSize"
    >
      {{ icon }}
    </ui-icon>
    
    <span v-if="avatar" class="chip__avatar">
      {{ avatar }}
    </span>
    
    <span class="chip__text">
      <slot>{{ text }}</slot>
    </span>
    
    <ui-icon
      v-if="removable"
      class="chip__remove"
      :size="iconSize"
      @click.stop="handleRemove"
    >
      close
    </ui-icon>
  </button>
</template>

<script setup>
import { computed } from 'vue';

const props = defineProps({
  /**
   * Text content to display in the chip
   */
  text: {
    type: String,
    default: '',
  },
  /**
   * Material icon name to display on the left
   */
  icon: {
    type: String,
    default: '',
  },
  /**
   * Avatar text/initials to display (alternative to icon)
   */
  avatar: {
    type: String,
    default: '',
  },
  /**
   * Whether the chip can be clicked
   */
  clickable: {
    type: Boolean,
    default: false,
  },
  /**
   * Whether the chip can be removed (shows close icon)
   */
  removable: {
    type: Boolean,
    default: false,
  },
  /**
   * Whether the chip is selected
   */
  selected: {
    type: Boolean,
    default: false,
  },
  /**
   * Use outlined variant instead of filled
   */
  outlined: {
    type: Boolean,
    default: false,
  },
  /**
   * Use dense/compact sizing
   */
  dense: {
    type: Boolean,
    default: false,
  },
  /**
   * Whether the chip is disabled
   */
  disabled: {
    type: Boolean,
    default: false,
  },
  /**
   * Custom color (CSS color value)
   */
  color: {
    type: String,
    default: '',
  },
  /**
   * Icon size
   */
  iconSize: {
    type: [String, Number],
    default: 18,
  },
});

const emit = defineEmits(['click', 'remove']);

const handleClick = (event) => {
  if (props.disabled) return;
  if (props.clickable) {
    emit('click', event);
  }
};

const handleRemove = (event) => {
  if (props.disabled) return;
  emit('remove', event);
};
</script>

<style scoped>
.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 16px;
  background-color: var(--mdc-theme-surface, #e0e0e0);
  color: var(--mdc-theme-on-surface, rgba(0, 0, 0, 0.87));
  font-size: 14px;
  font-family: Roboto, sans-serif;
  line-height: 1;
  white-space: nowrap;
  user-select: none;
  transition: background-color 0.2s, box-shadow 0.2s;
  height: fit-content;
  border: none;
}

.chip--dense {
  padding: 4px 8px;
  font-size: 13px;
  gap: 4px;
}

.chip--outlined {
  background-color: transparent;
  border: 1px solid var(--mdc-theme-on-surface, rgba(0, 0, 0, 0.12));
}

.chip--clickable {
  cursor: pointer;
}

.chip--clickable:hover:not(.chip--disabled) {
  background-color: var(--mdc-theme-primary);
  color: var(--mdc-theme-on-primary, white);
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
}

.chip--clickable:focus:not(.chip--disabled) {
  outline: none;
  /* background-color: var(--mdc-theme-primary); */
  /* color: var(--mdc-theme-on-primary, white); */
  box-shadow: 0 0 0 3px rgba(98, 0, 238, 0.3);
}

.chip--clickable:active:not(.chip--disabled) {
  background-color: color-mix(in srgb, var(--mdc-theme-primary) 85%, black);
  color: var(--mdc-theme-on-primary, white);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
  transform: translateY(1px);
}

.chip--outlined.chip--clickable:hover:not(.chip--disabled) {
  background-color: rgba(98, 0, 238, 0.08);
  border-color: var(--mdc-theme-primary);
  color: var(--mdc-theme-primary);
}

.chip--outlined.chip--clickable:focus:not(.chip--disabled) {
  outline: none;
  background-color: rgba(98, 0, 238, 0.12);
  border-color: var(--mdc-theme-primary);
  color: var(--mdc-theme-primary);
  box-shadow: 0 0 0 3px rgba(98, 0, 238, 0.2);
}

.chip--outlined.chip--clickable:active:not(.chip--disabled) {
  background-color: rgba(98, 0, 238, 0.18);
  border-color: color-mix(in srgb, var(--mdc-theme-primary) 85%, black);
  color: color-mix(in srgb, var(--mdc-theme-primary) 85%, black);
  transform: translateY(1px);
}

.chip--selected {
  background-color: var(--mdc-theme-primary);
  color: var(--mdc-theme-on-primary, white);
}

.chip--outlined.chip--selected {
  background-color: rgba(98, 0, 238, 0.12);
  border-color: var(--mdc-theme-primary);
  color: var(--mdc-theme-primary);
}

.chip--disabled {
  opacity: 0.38;
  cursor: default;
  pointer-events: none;
}

.chip__icon {
  flex-shrink: 0;
  margin-left: -4px;
}

.chip__avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background-color: var(--mdc-theme-primary);
  color: var(--mdc-theme-on-primary, white);
  font-size: 12px;
  font-weight: 500;
  margin-left: -6px;
  flex-shrink: 0;
}

.chip__text {
  flex: 1;
  /* overflow: hidden; */
  text-overflow: ellipsis;
}

.chip__remove {
  flex-shrink: 0;
  margin-right: -6px;
  cursor: pointer;
  opacity: 0.54;
  transition: opacity 0.2s;
}

.chip__remove:hover {
  opacity: 1;
}

/* Custom color support */
.chip[style*="--chip-color"] {
  background-color: var(--chip-color);
}

.chip--outlined[style*="--chip-color"] {
  background-color: transparent;
  border-color: var(--chip-color);
  color: var(--chip-color);
}
</style>
