<template>
  <router-link
    class="collection"
    :to="'/collections/' + collection.id"
  >
    {{ collection.name }}
  </router-link>
</template>

<script>
export default {
  props: ["collection"],
};
</script>

<style scoped>

.collection {
  max-width: 400px;
  margin: 10px;
  padding: 40px 30px;
  font-size: 1.4em;
  border-radius: 20px;
  color: var(--mdc-theme-on-surface);
  background: var(--mdc-theme-surface);
  text-decoration: none;
}

</style>