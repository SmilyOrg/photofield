<template>
  <div
    class="center-message centered"
  >
    <ui-icon class="icon">{{ icon }}</ui-icon>
    <div class="content">
      <slot></slot>
    </div>
  </div>
</template>

<script setup>

const { icon } = defineProps({
  icon: {
    type: String,
    default: "",
  },
});

</script>

<style scoped>

.icon {
  font-size: 100px;
  color: #ccc;
  margin-bottom: 20px;
}

.content {
  text-align: left;
  display: flex;
  flex-direction: column;
}

:deep(button) {
  margin: 20px auto;
}

.centered {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 20px;
  max-width: 800px;
  margin: auto;
  text-align: center;
  line-height: 1.5;
  height: 100%;
  flex-direction: column;
}

</style>
