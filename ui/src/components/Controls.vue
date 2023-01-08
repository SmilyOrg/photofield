<template>
  <div
    v-if="region"
    class="controls"
    :class="{ visible: !idle }"
  >
    <div class="nav exit" @click="exit()">
      <ui-icon light class="icon" size="32">arrow_back</ui-icon>
    </div>
    <!-- <div class="nav left" @click="left()">
      <ui-icon light class="icon" size="48">chevron_left</ui-icon>
    </div>
    <div class="nav right" @click="right()">
      <ui-icon light class="icon" size="48">chevron_right</ui-icon>
    </div> -->
  </div>
</template>

<script>
import { onKeyStroke, useIdle } from '@vueuse/core';

export default {

  props: {
    region: Object,
  },

  emits: ["navigate", "exit"],

  setup(_, { emit }) {

    const { idle } = useIdle(5000, {
      events: ["mousemove"],
      initialState: false,
    });

    const left = () => {
      emit("navigate", -1);
    }

    const right = () => {
      emit("navigate", 1);
    }

    const exit = () => {
      emit("exit");
    }

    onKeyStroke(["ArrowLeft"], left);
    onKeyStroke(["ArrowRight"], right);
    onKeyStroke(["Escape"], exit);

    return {
      idle,
      left,
      right,
      exit,
    }
  },
}

</script>

<style scoped>
.controls {
  position: relative;
  width: 100%;
  height: 100%;
  pointer-events: none;
}

.controls.visible .nav {
  opacity: 1;
  transition: none;
}

.nav {
  position: absolute;
  display: flex;
  cursor: pointer;
  
  align-items: center;
  
  -webkit-user-select: none;  
  -moz-user-select: none;    
  -ms-user-select: none;      
  user-select: none;

  pointer-events: all;

  opacity: 0;
  transition: opacity 1s cubic-bezier(0.47, 0, 0.745, 0.715);
}

.nav.left, .nav.right {
  width: 30%;
  height: 100%;
  margin-top: 64px;
}

.nav.visible {
  opacity: 1;
}

.nav.right {
  right: 0;
  flex-direction: row-reverse;
}

.nav .icon {
  margin: 20px;
  pointer-events: none;
  text-shadow: #000 0px 0px 2px;
  color: white;
}

.nav.left .icon, .nav.right .icon {
  margin-top: -64px;
}

</style>
