<template>
  <div class="video-player" v-if="add" v-show="show">
    <video
      ref="videoPlayer"
      class="video-js"
    ></video>
  </div>
</template>

<script>
module.exports = {

  props: {
    region: {
      type: Object,
    },
    size: {
      type: String,
    },
    params: {
      type: String,
    },
  },

  data() {
    return {
      options: {},
      add: true,
      show: false,
      // player: null
    };
  },

  watch: {
    region() {
      this.updatePlayer();
    },
    size() {
      this.updatePlayer();
    },
  },

  methods: {

    updatePlayer() {
      this.options = null;
      this.show = false;
      if (this.region) {
        this.options = {
          controls: this.size == "full",
          autoplay: true,
          loop: true,
          muted: this.size != "full",
          sources: [
            {
              src: `files/${this.region.data.id}/video/${this.size}/${this.region.data.filename}?${this.params}`,
              type: "video/mp4",
            },
          ],
        };
      }
      if (this.options) {
        if (this.player) {
          this.player.src(this.options.sources[0]);
          this.player.controls(this.options.controls);
          this.player.muted(this.options.muted);
          // this.player.autoplay(true);
          // this.destroyPlayer();
          // this.add = false;
          // await Vue.nextTick();
          // this.add = true;
          // await Vue.nextTick();
        } else {
          this.player = videojs(this.$refs.videoPlayer, this.options, () => {
            // this.show = true;
            // console.log(this.player);
          });
          this.player.on("play", () => {
            // console.log("play", this.player);
            this.show = true;
          });
        }
      } else {
        this.destroyPlayer();
      }
    },

    destroyPlayer() {
      if (this.player) {
        this.player.dispose();
      }
    },
    
  },

  mounted() {
    this.updatePlayer();
  },

  beforeDestroy() {
    this.destroyPlayer();
  },

};
</script>

<style scoped>
.video, .video-player, .video-js {
  width: 100%;
  height: 100%;
}
</style>
