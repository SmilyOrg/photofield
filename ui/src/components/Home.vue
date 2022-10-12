<template>
  <div class="container">
    <page-title :title="pageTitle"></page-title>
    
    <div class="collections">
      <collection-link
        class="collection"
        v-for="c in collections"
        :key="c.id"
        :collection="c"
      ></collection-link>
    </div>
  </div>
</template>

<script>
import { useApi } from '../api';
import PageTitle from './PageTitle.vue';
import CollectionLink from './CollectionLink.vue';

export default {

  components: {
    PageTitle,
    CollectionLink,
  },

  setup() {
    const { items: collections } = useApi(() => "/collections");

    return {
      collections,
    }
  },

  computed: {
    pageTitle() {
      return "Photos";
    },
  }

}
</script>

<style scoped>

.collections {
  margin: 20px;
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
}
</style>
