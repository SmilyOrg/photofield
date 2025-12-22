<template>
  <div class="image-chip" ref="imageChip" @mousedown="handleMouseDown">
    <chip
      clickable
      :selected="modelValue !== null"
      :removable="modelValue !== null"
      @click="handleChipClick"
      @remove="clear"
    >
      <div v-if="modelValue" class="chip-content">
        <img 
          class="thumbnail"
          :src="thumbnailUrl"
          alt="Reference image"
        />
        Image
      </div>
    </chip>
    <div
      v-if="showPreview"
      class="preview-container"
      @focusout="handleFocusOut"
      tabindex="-1"
    >
      <img 
        :src="previewUrl"
        alt="Preview image"
        class="preview-image"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue';
import Chip from './Chip.vue';
import { getPreviewUrl } from '../../api';

const props = defineProps({
  modelValue: {
    type: String,
    default: null,
  },
  icon: {
    type: String,
    default: 'image',
  },
  scene: {
    type: Object,
    default: null,
  },
});

const emit = defineEmits(['update:modelValue', 'change']);

const showPreview = ref(false);
const imageChip = ref(null);
const isMouseDownInComponent = ref(false);

const thumbnailUrl = computed(() => {
  if (!props.modelValue || !props.scene) {
    return 'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" width="24" height="24"%3E%3Crect fill="%23ccc" width="24" height="24"/%3E%3C/svg%3E';
  }

  const width = 20;
  const height = 20;
  return getPreviewUrl(props.modelValue, "img", {
    w: width,
    h: height,
  });
});

const previewUrl = computed(() => {
  if (!props.modelValue || !props.scene) {
    return '';
  }

  const width = 200;
  const height = 200;
  return getPreviewUrl(props.modelValue, "img", {
    w: width,
    h: height,
  });
});

const handleMouseDown = () => {
  isMouseDownInComponent.value = true;
  setTimeout(() => {
    isMouseDownInComponent.value = false;
  }, 0);
};

const handleChipClick = async () => {
  // If disabled (null), do nothing - we don't enable it without an ID
  if (props.modelValue === null) {
    return;
  }

  // Toggle the preview
  showPreview.value = !showPreview.value;
  
  // Wait for DOM update and focus the preview container
  if (showPreview.value) {
    await nextTick();
    if (imageChip.value) {
      const container = imageChip.value.querySelector('.preview-container');
      if (container) {
        container.focus();
      }
    }
  }
};

const handleFocusOut = (event) => {
  if (isMouseDownInComponent.value) {
    return;
  }
  
  if (!imageChip.value?.contains(event.relatedTarget)) {
    showPreview.value = false;
  }
};

const clear = () => {
  showPreview.value = false;
  emit('update:modelValue', null);
  emit('change', null);
};

watch(() => props.modelValue, (newValue) => {
  if (newValue === null) {
    showPreview.value = false;
  }
});
</script>

<style scoped>
.image-chip {
  position: relative;
  display: inline-flex;
  flex-direction: column;
  gap: 8px;
}

.chip-content {
  display: flex;
  align-items: center;
  gap: 8px;
}

.thumbnail {
  width: 20px;
  height: 20px;
  object-fit: cover;
  border-radius: 3px;
  background: var(--mdc-theme-surface);
}

.preview-container {
  min-width: 200px;
  background: var(--mdc-theme-surface);
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  padding: 8px;
  animation: slideDown 0.2s ease-out;
  outline: none;
}

.preview-image {
  display: block;
  max-width: 512px;
  max-height: 512px;
  width: auto;
  height: auto;
  border-radius: 4px;
}

@keyframes slideDown {
  from {
    opacity: 0;
    transform: translateY(-8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
