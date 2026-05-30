<template>
  <div class="photo-details" ref="container">
    <div class="background swipeable" ref="background"></div>
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
        v-if="tagsSupported"
        :tags="tags"
        :loading="loading"
        @add="addTag($event)"
        @remove="removeTag($event)"
      ></tags>
      <div v-if="faces.length > 0" class="faces-section">
        <div class="faces-label">
          <ui-icon class="faces-icon" outlined>face</ui-icon>
          <span class="faces-count">{{ faces.length }} face{{ faces.length === 1 ? '' : 's' }}</span>
        </div>
        <div class="faces-grid">
          <div
            v-for="face in faces"
            :key="face.id"
            class="face-crop"
            :style="getFaceCropStyle(face)"
          ></div>
        </div>
      </div>
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
    </dl>
  </div>
</template>

<script setup>
import { useSwipe } from '@vueuse/core';
import dateFormat from 'date-fns/format';
import dateParseISO from 'date-fns/parseISO';
import { computed, ref, toRefs } from 'vue';
import { useRegion, useRegionTags } from '../use';
import DetailItem from './DetailItem.vue';
import Map from './Map.vue';
import Tags from './Tags.vue';
import { useApi, getThumbnailUrl } from '../api';

const FACE_THUMB_SIZE = 72; // px, display size of each face crop
const FACE_BUFFER = 1.2;    // matches server-side faceBuffer in faces.go

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

const { data: capabilities } = useApi(() => "/capabilities");
const tagsSupported = computed(() => capabilities.value?.tags?.supported);

const background = ref(null);
const container = ref(null);

useSwipe(container, {
  onSwipeEnd(event, direction) {
    if (direction != "down") return;
    if (!event.target.closest('.swipeable')) {
      return;
    }
    emit("close");
  },
});

const {
  region,
  mutate: updateRegion,
  loading,
} = useRegion({ scene, id: regionId })

const {
  tags,
  add: addTag,
  remove: removeTag,
} = useRegionTags({ region, updateRegion });

const createdAt = computed(() => {
  const at = region.value?.data?.created_at;
  if (!at) return null;
  return dateParseISO(at);
});

const photo = computed(() => {
  return region.value?.data;
});

const faces = computed(() => {
  return photo.value?.faces ?? [];
});

const date = computed(() => {
  if (!createdAt.value) return "";
  return dateFormat(createdAt.value, "MMM d, yyyy");
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

// Finds the smallest available thumbnail for face crops
const smallestThumbnail = computed(() => {
  const thumbs = photo.value?.thumbnails;
  if (!thumbs || thumbs.length === 0) return null;
  return thumbs[0]; // already sorted smallest-first by server
});

function getFaceCropStyle(face) {
  const thumb = smallestThumbnail.value;
  if (!thumb || !photo.value) return {};

  const photoW = photo.value.width;
  const photoH = photo.value.height;
  if (!photoW || !photoH) return {};

  // Scale factor from original image to thumbnail
  const scale = thumb.width / photoW;

  // Crop region in original coords (matches server faceBuffer logic)
  const cropSize = Math.max(face.w, face.h) * FACE_BUFFER;
  const cropX = face.x + face.w * 0.5 - cropSize * 0.5;
  const cropY = face.y + face.h * 0.5 - cropSize * 0.5;

  // Crop region in thumbnail coords
  const tCropX = cropX * scale;
  const tCropY = cropY * scale;
  const tCropSize = cropSize * scale;

  // Scale the thumbnail so the crop fills FACE_THUMB_SIZE
  const displayScale = FACE_THUMB_SIZE / tCropSize;

  const bgWidth = Math.round(thumb.width * displayScale);
  const bgHeight = Math.round(thumb.height * displayScale);
  const bgOffsetX = Math.round(-tCropX * displayScale);
  const bgOffsetY = Math.round(-tCropY * displayScale);

  const url = getThumbnailUrl(photo.value.id, thumb.name, thumb.filename);

  return {
    width: `${FACE_THUMB_SIZE}px`,
    height: `${FACE_THUMB_SIZE}px`,
    backgroundImage: `url(${url})`,
    backgroundSize: `${bgWidth}px ${bgHeight}px`,
    backgroundPosition: `${bgOffsetX}px ${bgOffsetY}px`,
    backgroundRepeat: 'no-repeat',
  };
}

</script>

<style scoped>

.photo-details {
  background-color: var(--mdc-theme-background);
  max-width: 360px;
  max-height: 100%;
  overflow-y: auto;
  box-sizing: border-box;
  position: relative;
}

.swipeable, .swipeable * {
  overscroll-behavior: none;
  overscroll-behavior-block: none;
  overscroll-behavior-inline: none;
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

.bar > h2 {
  margin: 0;
}

.tags {
  padding: 0 18px;
  box-sizing: border-box;
}

.faces-section {
  padding: 12px 24px 16px;
}

.faces-label {
  display: flex;
  align-items: center;
  margin-bottom: 10px;
}

.faces-icon {
  width: 24px;
  height: 24px;
}

.faces-count {
  margin-left: 16px;
}

.faces-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.face-crop {
  border-radius: 4px;
  overflow: hidden;
  background-color: var(--mdc-theme-surface);
  flex-shrink: 0;
}

</style>