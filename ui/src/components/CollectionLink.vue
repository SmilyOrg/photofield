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
  border: rgb(51, 51, 51);
  border-radius: 20px;
  background: rgb(241, 241, 241);
  text-decoration: none;
  color: black;
}

</style>