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
import { getFileUrl, getVideoUrl } from '../api';
import Plyr from 'plyr';
import { isCloseClick } from '../utils';

const originalQualitySize = 1000000;

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
        options: [
          originalQualitySize,
          4320,
          2880,
          2160,
          1440,
          1080,
          720,
          576,
          480,
          360,
          240,
        ]
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
          this.region?.data?.thumbnails?.map(thumbnail => ({
            src: getVideoUrl(this.region.data.id, thumbnail.name, this.region.data.filename),
            size: thumbnail.height,
          })) || []
        ),
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
        this.show = true;
        this.player.toggleControls(false);
      }
    },
    onError(event) {
      console.error(event);
      if (!this.hasPlayed) {
        const qualityConfig = this.player.config.quality;
        const qualitySelected = qualityConfig.selected;
        const qualities = this.source.sources.map(source => source.size);
        const qualityIndex = qualities.findIndex(option => option == qualitySelected);
        if (qualityIndex < qualities.length - 1) {
          this.player.quality = qualities[qualityIndex + 1];
        }
      }
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
