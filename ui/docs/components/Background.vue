<template>
  <div class="background">
    <div class="plane">
      <img :src="src" alt="background image">
    </div>
  </div>
</template>

<script setup>
import { defineProps } from 'vue'

const props = defineProps({
  src: {
    type: String,
    required: true
  },
})
</script>

<style>
.background {
  position: fixed;
  top: -150vw;
  z-index: -10;
  image-rendering: pixelated;
  filter: blur(6px);
}

.background .plane { 
  transform:
    perspective(100vw)
    translateY(0px)
    rotateX(85deg)
    rotateY(0deg)
    rotateZ(10deg);
}

.background img {
  position: relative;
  top: 200px;
}

.background img {
  width: 100vw;
  filter: blur(2px);
  animation:
    moveDown 180s infinite linear,
    fadeOut 180s infinite linear;
}

.dark .background img {
  filter: blur(2px) invert(0.9);
}

@keyframes moveDown {
  0% {
    transform: translateY(-30%);
  }
  100% {
    transform: translateY(100%);
  }
}

@keyframes fadeOut {
  0% {
    opacity: 0;
  }
  1% {
    opacity: 1;
  }
  99% {
    opacity: 1;
  }
  100% {
    opacity: 0;
  }
}
</style>
