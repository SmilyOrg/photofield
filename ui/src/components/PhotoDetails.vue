<template>
  <div class="photo-details" ref="container">
    <div class="background" ref="background"></div>
    <div class="bar swipeable">
      <ui-icon-button
        icon="close"
        outlined
        @click="$emit('close')"
      >
      </ui-icon-button>
      <h2
        :class="$tt('headline5')"
      >
        Info
      </h2>
    </div>
    <dl class="contents" v-if="photo">
      <tags
        :region="region"
        :tags="region?.data?.tags"
        :loading="loading"
      ></tags>
      <detail-item
        icon="today"
        class="swipeable"
        :loading="loading"
        :title="date"
        :subtitles="[time, timezone]"
      ></detail-item>
      <detail-item
        icon="photo"
        class="swipeable"
        :loading="loading"
        :title="photo.filename"
        :subtitles="[megapixels, dimensions]"
        outlined
      ></detail-item>
      <detail-item
        v-if="photo.location"
        icon="room"
        class="swipeable"
        :loading="loading"
        :title="photo.location"
        outlined
      ></detail-item>
      <Map
        v-if="geoview"
        :loading="loading"
        class="map"
        :geoview="geoview"
      ></Map>
      <!-- <div class="thumbnails">
        <a
          class="thumbnail"
          v-for="thumb in region.data?.thumbnails"
          :key="thumb.name"
          :href="getThumbnailUrl(region.data.id, thumb.name, thumb.filename)"
          :title="thumb.width + ' x ' + thumb.height + ' (' + thumb.display_name + ')'"
          target="_blank"
        >
          {{ thumb.width }}
        </a>
      </div> -->
    </dl>
  </div>
</template>

<script setup>
import { useSwipe } from '@vueuse/core';
import dateFormat from 'date-fns/format';
import dateParseISO from 'date-fns/parseISO';
import { computed, ref, toRefs } from 'vue';
import { useRegion } from '../use';
import DetailItem from './DetailItem.vue';
import Map from './Map.vue';
import Tags from './Tags.vue';

const props = defineProps({
  scene: Object,
  regionId: String,
});

const emit = defineEmits([
  'close',
]);

const {
  scene,
  regionId,
} = toRefs(props);

const background = ref(null);
const container = ref(null);

useSwipe(container, {
  onSwipeEnd(event, direction) {
    if (direction != "DOWN") return;
    if (!event.target.closest('.swipeable')) {
      return;
    }
    emit("close");
  },
});

const {
  region,
  loading,
} = useRegion({ scene, id: regionId })

const createdAt = computed(() => {
  const at = region.value?.data?.created_at;
  if (!at) return null;
  return dateParseISO(at);
});

const photo = computed(() => {
  return region.value?.data;
});

const date = computed(() => {
  if (!createdAt.value) return "";
  return dateFormat(createdAt.value, "MMM d");
});

const time = computed(() => {
  if (!createdAt.value) return "";
  return dateFormat(createdAt.value, "EEE, h:mm aa");
});

const timezone = computed(() => {
  if (!createdAt.value) return "";
  return dateFormat(createdAt.value, "OOOO");
});

const megapixels = computed(() => {
  const width = photo.value?.width;
  const height = photo.value?.height;
  if (!width || !height) return "";
  return (width * height / 1e6).toFixed(1) + " MP";
});

const dimensions = computed(() => {
  const width = photo.value?.width;
  const height = photo.value?.height;
  if (!width || !height) return "";
  return width + " × " + height;
});

const geoview = computed(() => {
  if (!photo.value?.latlng) return null;
  return [
    photo.value.latlng.lng,
    photo.value.latlng.lat,
    12,
  ];
});

</script>

<style scoped>

.photo-details {
  background-color: white;
  max-width: 360px;
  max-height: 100%;
  overflow-y: auto;
  box-sizing: border-box;
}

.background {
  position: absolute;
  width: 100%;
  height: 100%;
  z-index: -1;
}

.bar {
  padding: 8px;
  display: flex;
  flex-direction: row;
  align-items: center;
}

.thumbnails {
  display: flex;
  flex-wrap: wrap;
  padding: 0 12px;
}

.thumbnail {
  font-size: 0.8em;
  padding: 10px 6px;
  text-decoration: none;
  color: var(--mdc-theme-text-primary-on-background);
}


.tags {
  padding: 0 18px;
  box-sizing: border-box;
}

.bar > h2 {
  margin: 0;
}

</style>