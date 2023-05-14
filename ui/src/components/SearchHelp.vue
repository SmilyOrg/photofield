<template>
  <div
    class="search-help"
    v-shadow="4"
  >
    <pre>{{ query }}</pre>

    <h2>Search examples</h2>

    <dl>
      <dt><i>a beautiful sunset</i></dt>
      <dd>A short, clear description of the image you're looking for.</dd>

      <dt><code>img:12345</code></dt>
      <dd>Images similar to the image with ID 12345.</dd>
    </dl>
  </div>
</template>

<script setup>
import { toRefs } from 'vue';
import { useApi } from '../api';

const props = defineProps({
  modelValue: String,
  loading: Boolean,
});

const {
  modelValue,
  loading,
} = toRefs(props);


const { data: query } = useApi(() => (
  "/search-queries/" + encodeURIComponent(modelValue.value)
));


</script>

<style scoped>

h2 {
  font-size: 1em;
}

.search-help {
  padding: 0 16px 0 16px;
  /* padding-bottom: 0; */
  background-color: var(--mdc-theme-background);
}

/* list item should show up as a thick line on the left with a certain primary color, like a highlighting marker */
.search-help ul {
  list-style-type: none;
  padding: 0;
  margin: 0;
}

dl {
  /* padding-left: 16px; */
}

dt {
  padding-left: 8px;
  border-left: 4px solid lightblue;
  margin-bottom: 8px;
}

code {
  font-size: 1.2em;
}

dd {
  margin-bottom: 20px;
  margin-left: 14px;
}

</style>