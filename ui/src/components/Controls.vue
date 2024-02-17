<template>
  <div
    class="controls"
    :class="{ visible: !idle || showTags }"
  >
    <div class="nav exit" @click="exit()">
      <ui-icon light class="icon" size="32">arrow_back</ui-icon>
    </div>
    <div class="toolbar">
      <!-- <ui-icon light class="icon" size="32" @click="$event => download()">download</ui-icon> -->
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
        @click="toggleFavorite()"
      >
        {{ favoriteTag ? "favorite" : "favorite_outline" }}
      </ui-icon>
    </div>
    <Tags
      v-if="showTags"
      class="tags"
      :region="region"
      :tags="region?.data?.tags"
      @add="addTag($event)"
      @remove="removeTag($event)"
    ></Tags>
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
import Tags from './Tags.vue';
import { computed, ref, toRefs } from 'vue';
import { postTagFiles, useApi } from '../api';
import { useRegion } from '../use';

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

const { data: capabilities } = useApi(() => "/capabilities");
const tagsSupported = computed(() => capabilities.value?.tags?.supported);

const fileId = computed(() => region.value?.data?.id);

const emit = defineEmits([
  "navigate",
  "exit",
]);

const { idle } = useIdle(5000, {
  events: ["mousemove"],
  initialState: false,
});

const showTags = ref(false);

const favoriteTag = computed(() => {
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

const toggleFavorite = async () => {
  const tagId = favoriteTag?.id || "fav:r0";
  if (!fileId.value) {
    return;
  }
  await postTagFiles(tagId, {
    op: "INVERT",
    file_id: fileId.value,
  });
  await updateRegion();
}

const addTag = async (tag) => {
  if (!fileId.value || !tag?.id) {
    return;
  }
  await postTagFiles(tag.id, {
    op: "ADD",
    file_id: fileId.value,
  });
  await updateRegion();
}

const removeTag = async (tag) => {
  if (!fileId.value || !tag?.id) {
    return;
  }
  await postTagFiles(tag.id, {
    op: "SUBTRACT",
    file_id: fileId.value,
  });
  await updateRegion();
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
