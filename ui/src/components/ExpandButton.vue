<template>
  <ui-button
    class="expand"
  >
    <ui-icon v-if="expanded">expand_less</ui-icon>
    <ui-icon v-if="!expanded">expand_more</ui-icon>
  </ui-button>
</template>

<script>
export default {
  props: ["expanded"],
};
</script>

<style scoped>

.expand {
  width: 100%;
  height: 20px;
}

</style>