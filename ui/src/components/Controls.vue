<template>
  <div
    class="controls"
    :class="{ visible: !idle }"
  >
    <div class="nav exit" @click="exit()">
      <ui-icon light class="icon" size="32">arrow_back</ui-icon>
    </div>
    <div class="toolbar">
      <!-- <ui-icon light class="icon" size="32" @click="$event => download()">download</ui-icon> -->
      <ui-icon light class="icon" size="32" @click="$event => onFavorite()">
        {{ favorite ? "favorite" : "favorite_outline" }}
      </ui-icon>
    </div>
    <!-- <Downloads class="downloads" :region="region"></Downloads> -->
    <!-- <div class="nav left" @click="left()">
      <ui-icon light class="icon" size="48">chevron_left</ui-icon>
    </div>
    <div class="nav right" @click="right()">
      <ui-icon light class="icon" size="48">chevron_right</ui-icon>
    </div> -->
  </div>
</template>

<script setup>
import { onKeyStroke, useIdle } from '@vueuse/core';
import Downloads from './Downloads.vue';
import { computed, toRefs } from 'vue';

const props = defineProps({
  region: Object,
});

const {
  region,
} = toRefs(props);

const emit = defineEmits([
  "navigate",
  "exit",
  "favorite",
]);

const { idle } = useIdle(5000, {
  events: ["mousemove"],
  initialState: false,
});

const favorite = computed(() => {
  return region.value?.data?.tags?.find(tag => tag.name == "fav");
})

const left = () => {
  emit("navigate", -1);
}

const right = () => {
  emit("navigate", 1);
}

const exit = () => {
  emit("exit");
}

const onFavorite = () => {
  emit("favorite", favorite.value);
}

onKeyStroke(["ArrowLeft"], left);
onKeyStroke(["ArrowRight"], right);
onKeyStroke(["Escape"], exit);

</script>

<style scoped>
.controls {
  position: relative;
  width: 100%;
  height: 100%;
  pointer-events: none;
  
  opacity: 0;
  transition: opacity 1s cubic-bezier(0.47, 0, 0.745, 0.715);
}

.controls.visible {
  opacity: 1;
  transition: none;
}

.nav {
  position: absolute;
  display: flex;
  cursor: pointer;
  
  align-items: center;
  
  -webkit-user-select: none;  
  -moz-user-select: none;    
  -ms-user-select: none;      
  user-select: none;

  pointer-events: all;
}

.controls .toolbar {
  position: absolute;
  display: flex;
  right: 0px;
  cursor: pointer;
  pointer-events: all;
  -webkit-user-select: none;
  -moz-user-select: none;
  -ms-user-select: none;
  user-select: none;
}

.nav.left, .nav.right {
  width: 30%;
  height: 100%;
  margin-top: 64px;
}

.nav.visible {
  opacity: 1;
}

.nav.right {
  right: 0;
  flex-direction: row-reverse;
}

.controls .icon {
  padding: 20px;
  text-shadow: #000 0px 0px 2px;
  color: white;
}

.downloads {
  position: absolute;
  top: 64px;
  right: 0;
  background-color: white;
  width: 100%;
  pointer-events: all;
}

.nav.left .icon, .nav.right .icon {
  margin-top: -64px;
}

</style>
