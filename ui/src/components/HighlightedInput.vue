<template>
  <div class="highlighted-input" :class="{ outlined }">
    <ui-textfield
      ref="textfieldRef"
      :modelValue="modelValue"
      :placeholder="placeholder"
      :outlined="outlined"
      @input="handleInput"
      v-bind="$attrs"
    />
    <div 
      ref="highlightLayer" 
      class="highlight-layer"
      v-html="highlightedHtml"
    ></div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';

/**
 * Props for HighlightedInput component
 * @typedef {Object} Token
 * @property {number} start - Start position in text (0-indexed)
 * @property {number} end - End position in text (exclusive)
 * @property {string} type - Token type for styling (e.g., 'qualifier', 'date', 'threshold')
 */

const props = defineProps({
  /** Input value (v-model) */
  modelValue: {
    type: String,
    default: ''
  },
  /** Array of tokens to highlight */
  tokens: {
    type: Array,
    default: () => []
  },
  /** Placeholder text */
  placeholder: {
    type: String,
    default: ''
  },
  /** Use outlined textfield style */
  outlined: {
    type: Boolean,
    default: false
  }
});

const emit = defineEmits(['update:modelValue']);

const textfieldRef = ref(null);
const highlightLayer = ref(null);
const inputEl = ref(null);
const lastScrollLeft = ref(0);
const rafId = ref(null);

const enabledTokenTypes = {
  qualifier: true,
};

// Extract native input element from BalmUI textfield and attach scroll listener
onMounted(async () => {
  await nextTick();
  if (textfieldRef.value && textfieldRef.value.$el) {
    inputEl.value = textfieldRef.value.$el.querySelector('input');
    if (inputEl.value) {
      inputEl.value.addEventListener('scroll', syncScroll);
      syncScroll();
    }
  }
});

// Cleanup scroll listener on unmount
onBeforeUnmount(() => {
  if (inputEl.value) {
    inputEl.value.removeEventListener('scroll', syncScroll);
  }
  if (rafId.value) {
    cancelAnimationFrame(rafId.value);
  }
});

// Sync scroll position with RAF throttling
const syncScroll = () => {
  if (rafId.value) return;

  rafId.value = requestAnimationFrame(() => {
    if (highlightLayer.value && inputEl.value) {
      const scrollLeft = inputEl.value.scrollLeft;
      if (scrollLeft !== lastScrollLeft.value) {
        highlightLayer.value.scrollLeft = scrollLeft;
        lastScrollLeft.value = scrollLeft;
      }
    }
    rafId.value = null;
  });
};

// Escape HTML for safe rendering
const escapeHtml = (text) => {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
};

// Generate highlighted HTML from tokens
const highlightedHtml = computed(() => {
  const text = props.modelValue;
  if (!text || props.tokens.length === 0) {
    // Return non-breaking space to maintain height
    return '&nbsp;';
  }

  const sorted = [...props.tokens].sort((a, b) => a.start - b.start);
  let html = '';
  let lastIndex = 0;

  sorted.forEach(token => {
    // Add text before token
    if (token.start > lastIndex) {
      html += escapeHtml(text.substring(lastIndex, token.start));
    }

    // Add highlighted token
    const tokenText = text.substring(token.start, token.end);
    if (enabledTokenTypes[token.type]) {
      html += `<mark class="token-${token.type}">${escapeHtml(tokenText)}</mark>`;
    } else {
      html += escapeHtml(tokenText);
    }

    lastIndex = token.end;
  });

  // Add remaining text
  if (lastIndex < text.length) {
    html += escapeHtml(text.substring(lastIndex));
  }

  return html || '&nbsp;';
});

// Handle input events
const handleInput = (event) => {
  emit('update:modelValue', event.target.value);
};

// Watch for text changes to sync scroll
watch(() => props.modelValue, async () => {
  syncScroll();
});

// Expose focus method for parent components
const focus = () => {
  if (textfieldRef.value && textfieldRef.value.$el) {
    const input = textfieldRef.value.$el.querySelector('input');
    if (input) input.focus();
  }
};

defineExpose({ focus, $el: textfieldRef });
</script>

<style scoped>
.highlighted-input {
  position: relative;
  display: inline-block;
  width: 100%;
}

.highlight-layer {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  pointer-events: none;
  overflow-x: auto;
  overflow-y: hidden;
  white-space: pre;
  word-wrap: break-word;
  
  /* Hide scrollbar while maintaining scroll capability */
  scrollbar-width: none; /* Firefox */
  -ms-overflow-style: none; /* IE/Edge */
  
  /* Match Material Design typography */
  font-family: var(--mdc-typography-subtitle1-font-family, var(--mdc-typography-font-family, Roboto, sans-serif));
  font-size: var(--mdc-typography-subtitle1-font-size, 1rem);
  font-weight: var(--mdc-typography-subtitle1-font-weight, 400);
  letter-spacing: var(--mdc-typography-subtitle1-letter-spacing, 0.009375em);
  
  /* Match textfield padding - will be adjusted based on outlined prop */
  padding: 14px 0;
  margin-top: -0.6px;
  
  user-select: none;

  color: transparent
}

/* Hide scrollbar for Chrome/Safari */
.highlight-layer::-webkit-scrollbar {
  display: none;
}

/* Make input background transparent so highlights show through */
.highlighted-input :deep(.mdc-text-field) {
  background-color: transparent;
  width: 100%;
  height: fit-content;
  padding: 0;
}

.highlighted-input :deep(.mdc-text-field__input) {
  background-color: transparent;
  position: relative;
  z-index: 1;
  padding: 14px 0;
}

/* Token type styles with CSS custom properties */
.highlight-layer :deep(mark) {
  --highlight-text-color: transparent;
  --highlight-shadow-color: transparent;
  --highlight-underline-color: color-mix(in oklab, var(--mdc-theme-primary) 10%, transparent);

  color: var(--highlight-text-color);
  /* text-decoration: solid underline 4px var(--highlight-underline-color);
  text-underline-offset: 2px; */
  /* background-color: transparent; */
  background-color: var(--highlight-underline-color);
  text-shadow:
    -1px -1px 0 var(--highlight-shadow-color),
     1px -1px 0 var(--highlight-shadow-color),
    -1px  1px 0 var(--highlight-shadow-color),
     1px  1px 0 var(--highlight-shadow-color);
}

.highlight-layer :deep(.token-qualifier) {
  --highlight-shadow-color: var(--mdc-theme-background);
  --highlight-text-color: var(--mdc-theme-primary);
}

</style>
