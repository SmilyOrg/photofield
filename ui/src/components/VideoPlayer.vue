<template>
  <div
    class="video-player"
    v-show="active && show"
  >
    <video
      ref="video"
      class="video"
      :style="{ width: '100%', height: '100%' }"
      autoplay
      loop
      controls
    >
    </video>
  </div>
</template>

<script>
import { getFileUrl, getThumbnailUrl } from '../api';
import Plyr from 'plyr';

const originalQualitySize = 1000000;
const qualities = [originalQualitySize, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1];

export default {

  props: {
    region: Object,
    full: Boolean,
    active: Boolean,
  },

  emits: ["interactive"],

  data() {
    return {
      show: false,
      loading: 0,
      hasPlayed: false,
      interactive: true,
    };
  },

  mounted() {
    this.player = new Plyr(this.$refs.video, {
      settings: ["captions", "quality", "speed", "loop"],
      muted: true,
      quality: {
        default: originalQualitySize,
        options: qualities,
      },
      i18n: {
        qualityLabel: {
          [originalQualitySize]: 'Original',
        },
      },
    });
    this.player.on("loadstart", this.onLoadStart);
    this.player.on("canplay", this.onCanPlay);
    this.player.on("playing", this.onPlaying);
    this.player.on("error", this.onError);
    this.player.elements.controls.addEventListener("pointerenter", this.onControlsPointerEnter);
    this.player.elements.controls.addEventListener("pointerleave", this.onControlsPointerLeave);
    this.player.source = this.source;
    console.log(this.player);
  },

  unmounted() {
    if (!this.player) return;
    this.player.elements.controls.removeEventListener("pointerenter", this.onControlsPointerEnter);
    this.player.elements.controls.removeEventListener("pointerleave", this.onControlsPointerLeave);
    this.player.off("loadstart", this.onLoadStart);
    this.player.off("canplay", this.onCanPlay);
    this.player.off("playing", this.onPlaying);
    this.player.off("error", this.onError);
    this.player.destroy();
    this.player = null;
  },

  computed: {
    source() {
      return {
        type: "video",
        sources: [
          {
            src: getFileUrl(this.region.data.id, this.region.data.filename),
            size: originalQualitySize,
          }
        ]
        .concat(
          this.region?.data?.thumbnails
            ?.filter(thumbnail => thumbnail.filename.endsWith(".mp4"))
            .sort((a, b) => b.height - a.height)
            .map((thumbnail, index) => ({
              src: getThumbnailUrl(this.region.data.id, thumbnail.name, thumbnail.filename),
              size: qualities.length - 1 - index,
              width: thumbnail.width,
              height: thumbnail.height,
            })) || []
        )
      }
    }
  },

  watch: {
    source: {
      immediate: true,
      handler(source) {
        if (!this.player) return;
        this.loading = 0;
        this.show = false;
        this.hasPlayed = false;
        this.player.source = source;
        if (this.player.quality != originalQualitySize) {
          this.player.quality = originalQualitySize;
        }
      },
    },
    active(active) {
      if (active) {
        this.player.play();
      } else {
        this.player.pause();
      }
    },
    interactive(interactive, prevInteractive) {
      if (interactive != prevInteractive) this.$emit("interactive", interactive);
    },
  },

  methods: {
    onLoadStart() {
      this.loading++;
    },
    onCanPlay() {
      this.loading--;
      if (this.loading < 0) this.loading = 0;
      if (this.loading === 0) {
        if (this.player.media.videoWidth > 0 && this.player.media.videoHeight > 0) {
          this.show = true;
        } else {
          console.warn("Video playback no image detected", this.player.media.videoWidth, this.player.media.videoHeight);
          this.nextQuality();
        }
      }
    },
    onError(event) {
      console.error("Video playback error", event);
      if (!this.hasPlayed) {
        this.nextQuality();
      }
    },
    nextQuality() {
      const config = this.player.config.quality;
      const selected = config.selected;
      const qualities = this.source.sources.map(source => source.size);
      const index = qualities.findIndex(option => option == selected);
      if (index < qualities.length - 1) {
        this.player.quality = qualities[index + 1];
      }
      console.log("Switching to next quality", qualities, selected, "->", this.player.quality)
    },
    onPlaying() {
      this.hasPlayed = true;
      this.show = true;
    },
    onControlsPointerEnter() {
      this.interactive = false;
    },
    onControlsPointerLeave() {
      this.interactive = true;
    },
  }

};
</script>

<style scoped>
.video-player, .video-player ::v-deep(.plyr) {
  width: 100%;
  height: 100%;
}
</style>
