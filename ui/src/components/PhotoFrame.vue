<template>
  <div
    class="photoframe"
    :class="{ visible: !!view }"
  >
    <div class="bar left"></div>
    <div class="bar right"></div>
    <div class="bar top"></div>
    <div class="bar bottom"></div>
  </div>
</template>

<script setup>
import { computed, toRefs } from 'vue';

const props = defineProps({
  region: Object,
  view: Object,
});

const {
  view,
} = toRefs(props);

const e = 0.5;
const barLeft = computed(() => 
  Math.ceil(view.value?.x + e) + 'px'
);
const barRight = computed(() => 
  Math.floor(view.value?.x + view.value?.w - e) + 'px'
);
const barTop = computed(() =>
  Math.ceil(view.value?.y + e) + 'px'
);
const barBottom = computed(() =>
  Math.floor(view.value?.y + view.value?.h - e) + 'px'
);

</script>

<style scoped>
.photoframe {
  width: 100%;
  height: 100%;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.5s ease-in-out;
}

.photoframe.visible {
  opacity: 1;
}

.bar {
  position: absolute;
  background-color: black;
  pointer-events: none;
}

.bar.left {
  left: 0;
  top: 0;
  width: v-bind('barLeft');
  height: 100%;
}

.bar.right {
  right: 0;
  top: 0;
  width: calc(100% - v-bind('barRight'));
  height: 100%;
}

.bar.top {
  left: 0;
  width: 100%;
  top: 0;
  height: v-bind('barTop');
}

.bar.bottom {
  left: 0;
  width: 100%;
  bottom: 0;
  height: calc(100% - v-bind('barBottom'));
}

</style>
