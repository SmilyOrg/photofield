<template>
  <span>{{ num }} {{ suffix }}</span>>
</template>

<script setup>
import { computed, toRefs } from 'vue';


const props = defineProps({
    count: Number,
});

const {
  count,
} = toRefs(props);

const suffixes = ["px", "KP", "MP", "GP"];

const magnitude = computed(() => {
  let num = count.value;
  for (let i = 0; i < suffixes.length; i++) {
    if (num < 1000) return i;
    num /= 1000;
  }
  return i;
})

const suffix = computed(() => suffixes[magnitude.value]);

const num = computed(() => {
  console.log(count.value)
  return count.value / Math.pow(10, 3*magnitude.value);
})

</script>

<style scoped>

</style>