<template>
  <ui-icon-button
    :icon="
      colorMode == 'light' ? 'light_mode' :
      colorMode == 'dark' ? 'dark_mode' :
      'brightness_auto'
    "
    @click="
      colorMode = (
        colorMode == 'light' ? 'auto' :
        colorMode == 'dark' ? 'light' :
        'dark'
      )
    "
    outlined
  >
  </ui-icon-button>
</template>

<script setup>
import { useColorMode } from '@vueuse/core';

const { store: colorMode } = useColorMode();
</script>