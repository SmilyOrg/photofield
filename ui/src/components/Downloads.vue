<template>
  <div class="downloads">
    <ui-nav>
      <ui-nav-item
        v-for="thumb in region.data?.thumbnails"
        :key="thumb.name"
        class="thumbnail"
        :href="getThumbnailUrl(region.data.id, thumb.name, thumb.filename)"
        target="_blank"
      >
        {{ thumb.width }} x {{ thumb.height }} ({{ thumb.display_name }})
      </ui-nav-item>
    </ui-nav>
  </div>
</template>

<script setup>
import { toRefs } from 'vue';
import { getThumbnailUrl } from '../api';

const props = defineProps({
    region: Object,
});

const {
  region,
} = toRefs(props);

</script>

<style scoped>

.downloads {
  background-color: white;
  max-width: 360px;
  max-height: 100%;
  overflow-y: auto;
}

</style>