<template>
  <ui-card
    class="region"
    v-if="region && region.data"
    :style="{ width: menuWidth + 'px' }"
    :class="{ right: flipX, bottom: flipY, contracted: !expanded }"
  >
    <ui-card-media v-if="expanded">
      <tile-viewer
        class="viewer"
        ref="viewer"
        :scene="scene"
        :view="region.bounds"
        :viewport="viewport"
        :tileSize="tileSize"
        :style="{ height: imageHeight + 'px' }"
      ></tile-viewer>
      <ui-card-media-content>
        <div class="id pill">{{ region.data.id }}</div>
        <div class="filename">
          {{ region.data.filename }}
        </div>
      </ui-card-media-content>
    </ui-card-media>


    <ui-card-actions class="actions">
      <ui-nav class="nav">
        <ui-nav-item
          :href="fileUrl"
          target="_blank"
          @click="$emit('close')"
        >
          Open Image in New Tab
        </ui-nav-item>
        <ui-item @click="copyImage()">
          Copy Image
        </ui-item>
        <ui-item @click="copyImageLink()">
          Copy Image Link
        </ui-item>
      </ui-nav>
      <div v-if="expanded" class="thumbnails">
        <a
          v-for="thumb in region.data?.thumbnails"
          :key="thumb.name"
          class="thumbnail"
          :href="getThumbnailUrl(region.data.id, thumb.name, thumb.filename)"
          target="_blank"
        >
          {{ thumb.width }}
        </a>
      </div>
      <expand-button
        :expanded="expanded"
        @click="expanded = !expanded"
      ></expand-button>
    </ui-card-actions>
    <!-- <pre>{{ region }}</pre> -->
  </ui-card>
</template>

<script>
import copyImg from 'copy-image-clipboard';

import TileViewer from './TileViewer.vue';
import ExpandButton from './ExpandButton.vue';
import { getFileUrl, getThumbnailUrl } from '../api';
import { ref } from 'vue';
import { useViewport } from '../use';

export default {
  props: ["region", "scene", "flipX", "flipY", "tileSize"],
  emits: ["close"],
  components: { TileViewer, ExpandButton },
  data() {
    return {
      menuWidth: 240,
      expanded: false,
    }
  },
  setup() {
    const viewer = ref(null);
    const viewport = useViewport(viewer);
    return {
      viewer,
      viewport,
    }
  },
  computed: {
    fileUrl() {
      const data = this.region?.data;
      if (!data || !data.id || !data.filename) return null;
      return getFileUrl(data.id, data.filename);
    },
    imageHeight() {
      const bounds = this.region?.bounds;
      if (!bounds || !bounds.w || !bounds.h) return 200;
      return Math.min(200, this.menuWidth/bounds.w*bounds.h);
    },
    fillBounds() {
      const bounds = this.region.bounds;
      const scale = 0.5;
      return {
        x: bounds.x + bounds.w * (0.5 - 0.5 * scale),
        y: bounds.y + bounds.h * (0.5 - 0.5 * scale),
        w: bounds.w * scale,
        h: bounds.h * scale,
      }
    }
  },
  methods: {
    getFileUrl,
    getThumbnailUrl,
    async copyImage() {
      const id = this.region?.data?.id;
      if (!id) return;
      await copyImg(this.fileUrl);
      this.$emit("close");
    },
    async copyImageLink() {
      await navigator.clipboard.writeText(this.fileUrl);
      this.$emit("close");
    },
  }
};
</script>

<style scoped>

.region {
  position: absolute;
  background: white;
  box-shadow: #ffffff54 0 0 17px 15px;
}

.region.bottom {
  bottom: 10px;
}

.region.right {
  right: 10px;
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

.thumbnail:hover {
  background: rgb(241, 241, 241);
}

.filename {
  position: absolute;
  bottom: 0;
  margin: 0px;
  padding: 10px;
  width: calc(100% - 20px);
  color: white;
  background: linear-gradient(180deg, transparent, #0000007a);
}

.id {
  position: absolute;
  top: 8px;
  right: 8px;
}

.pill {
  background: rgba(0, 0, 0, 0.089);
  color: rgb(255, 255, 255);
  border-radius: 6px;
  padding: 2px 4px;
  font-size: 0.9em;
}

.id:hover {
  background: rgb(77, 77, 77);
  color: white;
}

.actions {
  flex-direction: column;
  align-items: flex-start;
  padding: 0;
}

.actions .nav {
  width: 100%;
}

.actions .icon {
  margin-left: auto;
  padding-left: 6px;
}

.link {
  width: 100%;
}

.viewer {
  width: 100%;
}

.tiny {
  width: 48px;
  height: 48px;
  margin-right: 16px;
  border-top-left-radius: 6px;
  overflow: hidden;
}

</style>