<script>
module.exports = {
  name: "page-title",
  props: ["title"],
  watch: {
    title: {
      immediate: true,
      handler() {
        document.title = this.title;
      },
    },
  },
  render() {},
};
</script>
