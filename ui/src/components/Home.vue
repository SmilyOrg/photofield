<template>
  <div class="container">
    <page-title :title="pageTitle"></page-title>
    
    <div class="collections">
      <collection-panel
        class="collection"
        v-for="c in collections"
        :key="c.id"
        :collection="c"
      ></collection-panel>
    </div>
  </div>
</template>

<script>
import { useApi } from '../api';
import PageTitle from './PageTitle.vue';
import CollectionPanel from './CollectionPanel.vue';

export default {

  components: {
    PageTitle,
    CollectionPanel,
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
