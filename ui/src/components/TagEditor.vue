<template>
  <div class="tag-editor">
    
    <tags
      class="add-tag"
      :tags="preview"
      @add="addTag"
      @remove="removeTag"
    ></tags>

    {{ title }}
    
    <table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Files</th>
          <th>Share</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="tag in workingCopy"
          :key="tag.name"
          :class="{
            'to-remove': tag.toRemove,
            'to-add': tag.toAdd,
          }"
        >
          <td>
            <span class="multiselect__tag">
              {{ tag.name }}
            </span>
          </td>
          <td class="right">
            {{ tag.file_count }}
          </td>
          <td class="right">
            {{ (tag.file_count / fileCount * 100).toFixed(0) }}%
          </td>
          <td class="actions">
            <ui-icon-button
              :icon="tag.toRemove || tag.toAdd ? 'undo' : 'delete'"
              @click="tag.toRemove ? addTag(tag) : removeTag(tag)"
            >
            </ui-icon-button>
          </td>
        </tr>
      </tbody>
    </table>
    
    <div v-if="toAdd.size === 0 && toRemove.size === 0">
      <div class="main-actions">
        No changes pending
      </div>
    </div>
    <div v-else>
      <h3>Pending changes</h3>
      <ol>
        <li v-for="tag in toAdd.values()" :key="tag.name" class="to-add">
          Add <span class="multiselect__tag">{{ tag.name }}</span> to {{ tag.file_count }} file{{ tag.file_count > 1 ? "s" : "" }}
        </li>
        <li v-for="tag in toRemove.values()" :key="tag.name" class="to-remove">
          Remove <span class="multiselect__tag">{{ tag.name }}</span> from {{ tag.file_count }} file{{ tag.file_count > 1 ? "s" : "" }}
        </li>
      </ol>
      <div class="main-actions">
        <ui-button @click="clear()">Clear</ui-button>
        <ui-button raised @click="apply()">Apply</ui-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, toRefs, watchEffect } from 'vue';
import { get, postTagFiles, useApi, useScene } from '../api';
import Tags from './Tags.vue';

const props = defineProps({
  tagId: String,
});

const emit = defineEmits({
  title: null,
});

const { tagId } = toRefs(props);

const tagsResponse = useApi(() => {
  if (!tagId.value) return "/tags";
  return `/tags/${tagId.value}/files-tags`;
});
const { data, items: tags, itemsMutate: refreshTags } = tagsResponse;

const workingCopy = computed(() => {
  return tags.value
    ?.map(tag => {
      if (toRemove.value.has(tag.name)) {
        return {
          ...tag,
          toRemove: true,
        };
      }
      return tag;
    })
    .concat(
      Array.from(toAdd.value.values()).map(tag => {
        return {
          ...tag,
          toAdd: true,
        };
      })
    )
    .filter(tag => tag !== null);
});

const preview = computed(() => {
  return workingCopy.value?.filter(tag => !tag.toRemove)
});

const selection = computed(() => {
  return tagId.value?.startsWith("sys:select:");
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
  return `The selection of ${fileCount.value} files is also tagged with the following tags`;
});

watchEffect(() => {
  emit("title", title.value);
});

const toAdd = ref(new Map());
const toRemove = ref(new Map());

const addTag = (tag) => {
  const name = tag.name;
  toRemove.value.delete(name);
  if (tags.value.find(tag => tag.name === name)) return;
  tag.file_count = fileCount.value;
  toAdd.value.set(name, tag);
  console.log("add", name, toAdd.value);
}

const removeTag = (tag) => {
  const name = tag.name;
  toAdd.value.delete(name);
  const existing = tags.value.find(tag => tag.name === name);
  if (!existing) return;
  toRemove.value.set(name, existing);
  console.log("remove", name, toRemove.value);
}

const clear = () => {
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

const addTag2 = async (tag) => {
  await postTagFiles(tag.id, {
    op: "ADD",
    tag_id: tagId.value,
  });
  await refreshTags();
}

const removeTag2 = async (tag) => {
  await postTagFiles(tag.id, {
    op: "SUBTRACT",
    tag_id: tagId.value,
  });
  await refreshTags();
}
</script>

<style scoped>
.tag-editor {
  position: relative;
  margin: 0 20px;
  display: flex;
  flex-direction: column;
  gap: 1em;
}

table {
  width: 100%;
  border-collapse: collapse;
  /* transition: background-color 0.2s; */
}

.add-tag {
  position: sticky;
  top: 10px;
  margin-bottom: 10px;
  background-color: white;
  z-index: 10;
}

td {
  padding: 0 1em;
}

tr {
  transition: background-color 0.2s;
}

.multiselect__tag {
  padding-right: 10px;
  font-weight: normal;
  margin: 0;
  vertical-align: bottom;
}

table .multiselect__tag {
  vertical-align: middle;
}

.to-remove, .to-add {
  font-weight: bold;
}

li {
  padding: 0.5em;
  margin: 0.5em 0;
}

.to-remove {
  background-color: #ffd7d7; /* Set the background color for rows to be deleted */
}

.to-add {
  background-color: #d7ffd7; /* Set the background color for rows to be added */
}

.min-width {
  min-width: 300px;
}

.right {
  text-align: right;
}

.actions {
  text-align: right;
}

.main-actions {
  display: flex;
  gap: 16px;
  justify-content: flex-end;
}

.hidden {
  visibility: hidden;
}

</style>