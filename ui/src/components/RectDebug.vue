<template>
  <canvas ref="canvas" :width="drawWidth" :height="drawHeight"></canvas>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue';
import { defineProps } from 'vue';

const props = defineProps({
  rectangles: {
    type: Array,
    required: true,
    validator: (rectangles) => {
      return rectangles.every(rect => 
        'x' in rect && 'y' in rect && 'w' in rect && 'h' in rect && 'color' in rect
      );
    }
  },
  width: {
    type: Number,
    default: 500
  },
  height: {
    type: Number,
    default: 500
  },
  drawWidth: {
    type: Number,
    default: 500
  },
  drawHeight: {
    type: Number,
    default: 500
  }
});

const canvas = ref(null);

const drawRectangles = () => {
  if (!canvas.value) return;
  const ctx = canvas.value.getContext('2d');
  ctx.clearRect(0, 0, props.drawWidth, props.drawHeight);

  const scale = Math.min(props.drawWidth / props.width, props.drawHeight / props.height);

  const offsetX = (props.drawWidth - props.width * scale) / 2;
  const offsetY = (props.drawHeight - props.height * scale) / 2;

  props.rectangles.forEach(rect => {
    ctx.fillStyle = rect.color;
    ctx.fillRect(
      rect.x * scale + offsetX,
      rect.y * scale + offsetY,
      Math.ceil(rect.w * scale),
      Math.ceil(rect.h * scale)
    );
  });
};

watch(() => props.rectangles, drawRectangles, { deep: true });

onMounted(drawRectangles);
</script>

<style scoped>
canvas {
  background: #fafafa;
}
</style>
