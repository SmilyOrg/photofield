<script>
export default {
  props: ["title"],
  watch: {
    title: {
      immediate: true,
      handler(title) {
        if (!title) return;
        document.title = title;
      },
    },
  },
  render() {},
};
</script>
