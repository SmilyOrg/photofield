<template>
  <div class="tags">
    <VueMultiselect
      v-model="tags"
      :options="options"
      :multiple="true"
      :taggable="true"
      tag-position="bottom"
      track-by="name"
      label="name"
      :loading="loading"
      :close-on-select="false"
      :clear-on-select="false"
      placeholder="Add tags"
      select-label="Select"
      selected-label="Selected"
      deselect-label="Remove"
      tag-placeholder="New tag"
      @search-change="onSearch"
      @tag="add"
      @select="select"
      @remove="remove"
    ></VueMultiselect>
  </div>
</template>

<script setup>
import { ref, toRefs } from 'vue';
import VueMultiselect from 'vue-multiselect'
import { get } from '../api';
import qs from "qs";

const props = defineProps({
  tags: Array,
});

const emit = defineEmits([
  "add",
  "remove",
]);

const {
  tags,
} = toRefs(props);

const options = ref([])
const loading = ref(false);


const onSearch = async (query) => {
  loading.value = true;
  const tags = await get(`/tags?${qs.stringify({ q: query })}`);
  loading.value = false;
  options.value = tags?.items;
}

const add = (newTag) => {
  const tagId = newTag + ":r0";
  emit("add", tagId);
}

const select = (tag) => {
  emit("add", tag.id);
}

const remove = (tag) => {
  emit("remove", tag.id);
}

</script>