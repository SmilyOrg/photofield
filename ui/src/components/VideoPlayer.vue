<template>
  <div
    class="video-player"
    v-show="active && show"
    @pointerDown.capture="onPointerDown"
    @click.capture="onCaptureClick"
    @wheel="onWheel"
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
import { isCloseClick } from '../utils';

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
      interactive: false,
    };
  },

  mounted() {
    this.player = new Plyr(this.$refs.video, {
      settings: ["captions", "quality", "speed", "loop"],
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
    this.player.on("ready", this.onReady);
    this.player.on("loadstart", this.onLoadStart);
    this.player.on("canplay", this.onCanPlay);
    this.player.on("playing", this.onPlaying);
    this.player.on("play", this.onPlay);
    this.player.on("pause", this.onPause);
    this.player.on("error", this.onError);

    // this.player.on("ready", () => console.log("ready"));
    // this.player.on("loadstart", () => console.log("loadstart"));
    // this.player.on("loadeddata", () => console.log("loadeddata"));
    // this.player.on("loadedmetadata", () => console.log("loadedmetadata"));
    // this.player.on("qualitychange", () => console.log("qualitychange"));
    // this.player.on("canplay", () => console.log("canplay"));
    // this.player.on("canplaythrough", () => console.log("canplaythrough"));
    // this.player.on("play", () => console.log("play"));
    // this.player.on("pause", () => console.log("pause"));
    // this.player.on("stalled", () => console.log("stalled"));
    // this.player.on("waiting", () => console.log("waiting"));
    // this.player.on("emptied", () => console.log("emptied"));
    // this.player.on("error", () => console.log("error"));
    // this.player.on("playing", () => console.log("playing"));

    this.player.source = this.source;
  },

  unmounted() {
    if (!this.player) return;
    this.player.off("ready", this.onReady);
    this.player.off("loadstart", this.onLoadStart);
    this.player.off("canplay", this.onCanPlay);
    this.player.off("playing", this.onPlaying);
    this.player.off("play", this.onPlay);
    this.player.off("pause", this.onPause);
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
      this.interactive = false;
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
    onPlay() {
      // TODO: The locking of controls here does not work that well yet, so it's
      // disabled for now
      // this.interactive = true;
    },
    onPause() {
      this.interactive = false;
    },
    onPointerDown(event) {
      this.lastPointerDownEvent = event;
    },
    onCaptureClick(event) {
      if (!isCloseClick(this.lastPointerDownEvent, event)) {
        // Do not play/pause video if the click was a drag instead of a click
        event.stopPropagation();
      }
    },
    onWheel() {
      this.interactive = false;
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
