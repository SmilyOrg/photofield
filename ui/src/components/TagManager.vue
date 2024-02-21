<template>
  <div class="tag-manager">

    <response-loader
      class="response"
      :response="tagsResponse"
    ></response-loader>

    <tile-viewer
      class="viewer list"
      :scene="scene"
      :tileSize="512"
      :interactive="true"
      :pannable="true"
      :zoomable="true"
      :viewport="viewport"
      :selectTagId="tagId"
    ></tile-viewer>

    <div class="list" v-if="tags">
      <span v-if="pending">Tags*</span>
      <span v-else>Tags</span>
      <tags
        :tags="workingCopy"
        @add="addTag"
        @remove="removeTag"
      ></tags>
    </div>

    <div class="list">
      <div class="list" v-if="toRemove.size > 0">
        <span>
          Remove
        </span>
        <tags
          v-if="toRemove.size > 0"
          :tags="Array.from(toRemove.values())"
          :readonly="true"
        ></tags>
      </div>

      <div class="list" v-if="toAdd.size > 0">
        <span>
          Add
        </span>
        <tags
          v-if="toAdd.size > 0"
          :tags="Array.from(toAdd.values())"
          :readonly="true"
        ></tags>
      </div>

      <div class="actions clear" v-if="pending">
        <ui-button raised @click="apply">
          Apply to {{ fileCount }} files
        </ui-button>

        <ui-button outlined @click="cancel">
          Cancel
        </ui-button>
      </div>

    </div>

  </div>
</template>

<script setup>
import { computed, ref, toRefs, watchEffect } from 'vue';
import { postTagFiles, useApi, useScene } from '../api';
import Tags from './Tags.vue';
import TileViewer from './TileViewer.vue';
import ResponseLoader from './ResponseLoader.vue';

const props = defineProps({
  tagId: String,
});

const emit = defineEmits({
  title: null,
});

const {
  tagId
} = toRefs(props);

const tagsResponse = useApi(() => {
  if (!tagId.value) return "/tags";
  return `/tags/${tagId.value}/files-tags`;
});
const {
  data,
  items: tags,
  itemsMutate: refreshTags,
} = tagsResponse;

const viewport = {
  width: ref(300),
  height: ref(300),
};

const { scene } = useScene({
  layout: ref("WALL"),
  collectionId: ref("2020-10-ruhrgebiet"),
  imageHeight: ref(100),
  search: computed(() => `tag:${tagId.value}`),
  viewport,
});

const toAdd = ref(new Map());
const toRemove = ref(new Map());

const selection = computed(() => {
  return tagId.value?.startsWith("sys:select:");
});

const selectionCollectionId = computed(() => {
  if (!selection.value) return;
  return tagId.value.split(":")[3];
});

const fileCount = computed(() => {
  return data.value?.file_count;
});

const title = computed(() => {
  if (fileCount.value == null) {
    if (selection.value) return "Selection";
    return "Tags";
  }
  if (!selection.value) return "Tags";
  return `Selection of ${fileCount.value} files`;
});

watchEffect(() => {
  emit("title", title.value);
});

const pending = computed(() => {
  return toAdd.value.size > 0 || toRemove.value.size > 0;
});

const workingCopy = computed(() => {
  return tags.value
  ?.map(
    tag => (
      !selectionCollectionId.value?
      tag :
      {
        ...tag,
        route: {
          path: "/collections/" + selectionCollectionId.value,
          query: {
            search: `tag:${tag.name}`,
          },
        },
      }
    )
  )
  .filter(
    tag => !toRemove.value.has(tag.name)
  )
  .concat(
    Array.from(toAdd.value.values())
  );
});

const addTag = (tag) => {
  const name = tag.name;
  toRemove.value.delete(name);
  if (tags.value.find(tag => tag.name === name)) return;
  toAdd.value.set(name, tag);
  console.log("add", name, toAdd.value);
}

const removeTag = (tag) => {
  const name = tag.name;
  toAdd.value.delete(name);
  if (!tags.value.find(tag => tag.name === name)) return;
  toRemove.value.set(name, tag);
  console.log("remove", name, toRemove.value);
}

const cancel = () => {
  toAdd.value.clear();
  toRemove.value.clear();
}

const apply = async () => {
  
  const add = Array.from(toAdd.value.values());
  const remove = Array.from(toRemove.value.values());

  console.log("apply", toAdd, toRemove);
  
  for (const tag of remove) {
    await postTagFiles(tag.id, {
      op: "SUBTRACT",
      tag_id: tagId.value,
    });
    toRemove.value.delete(tag.name);
    await refreshTags();
  }

  for (const tag of add) {
    await postTagFiles(tag.id, {
      op: "ADD",
      tag_id: tagId.value,
    });
    toAdd.value.delete(tag.name);
    await refreshTags();
  }
}

</script>

<style scoped>
.tag-manager {
  margin: 0 20px;
}

.inline {
  display: inline-block;
}

.viewer {
  max-width: 300px;
  height: 300px;
}

.cols {
  display: flex;
  /* flex-direction: row; */
  /* gap: 20px; */
  /* align-items: center; */
  flex-wrap: wrap;
  min-width: 300px;
}

.list {
  float: left;
  margin: 0 20px 20px 0;
  /* display: inline-block; */
  /* flex-direction: column; */
  /* margin-bottom: 20px; */
  max-width: 300px;
  gap: 8px;
}

.actions {
  display: flex;
  gap: 8px;
}

.clear {
  clear: both;
}

</style>