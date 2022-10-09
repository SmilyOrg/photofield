<template>
  <div class="skeleton">
    <div
      class="pattern"
      :style="{
        backgroundPositionX: -offset.x + 'px',
        backgroundPositionY: -offset.y + 'px',
      }"
    ></div>
  </div>
</template>

<script>
export default {
  props: ["offset"],
};
</script>

<style scoped>

.skeleton {
  background-color: #ffffff;
}

.pattern {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background-image:
    linear-gradient(#ffffff 4px, transparent 4px),
    linear-gradient(to right, #ffffff 4px, #000000 4px);
  background-size: 300px 200px;
  animation: shine 2s linear infinite;
}

@keyframes shine {
  0% {
    opacity: 0.1;
  }
  70% {
    opacity: 0.1;
  }
  80% {
    opacity: 0.14;
  }
  100% {
    opacity: 0.1;
  }
}

</style>