<template>
  <div
    class="controls"
    :class="{ visible: !idle || showTags }"
  >
    <div class="nav exit" @click="exit()">
      <ui-icon light class="icon" size="32">arrow_back</ui-icon>
    </div>
    <div class="toolbar">
      <ui-icon
        light
        class="icon"
        size="32"
        @click="emit('info')"
      >
        info_outline
      </ui-icon>
      <ui-icon
        v-if="tagsSupported"
        light
        class="icon"
        size="32"
        @click="showTags = !showTags"
      >
        tag
      </ui-icon>
      <ui-icon
        v-if="tagsSupported"
        light
        class="icon"
        size="32"
        @click="invertTag('fav')"
      >
        {{ favoriteTag ? "favorite" : "favorite_outline" }}
      </ui-icon>
    </div>
    <Tags
      v-if="showTags"
      class="tags"
      :tags="tags"
      @add="addTag($event)"
      @remove="removeTag($event)"
    ></Tags>
  </div>
</template>

<script setup>
import { onKeyStroke, useIdle } from '@vueuse/core';
import Tags from './Tags.vue';
import { computed, ref, toRefs } from 'vue';
import { useApi } from '../api';
import { useRegion, useRegionTags } from '../use';

const props = defineProps({
  scene: Object,
  regionId: String,
});

const {
  scene,
  regionId,
} = toRefs(props);

const {
  region,
  mutate: updateRegion,
} = useRegion({ scene, id: regionId })

const {
  tags,
  add: addTag,
  remove: removeTag,
  invert: invertTag,
} = useRegionTags({ region, updateRegion });

const { data: capabilities } = useApi(() => "/capabilities");
const tagsSupported = computed(() => capabilities.value?.tags?.supported);

const emit = defineEmits([
  "navigate",
  "exit",
  "info",
]);

const { idle } = useIdle(5000, {
  events: ["mousemove"],
  initialState: false,
});

const showTags = ref(false);

const favoriteTag = computed(() => {
  return tags.value?.find(tag => tag.name == "fav");
})

const left = e => {
  if (e.altKey) return;
  emit("navigate", -1);
}

const right = e => {
  if (e.altKey) return;
  emit("navigate", 1);
}

const exit = (e) => {
  emit("exit");
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

.tags {
  position: absolute;
  top: 60px;
  right: 10px;
  pointer-events: all;
  width: 300px;
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
