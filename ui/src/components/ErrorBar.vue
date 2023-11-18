<template>
  <div
    class="error-bar"
    v-shadow="4"
  >
    <ui-collapse ripple class="collapse">
      <template #toggle>
        <div class="message">
          <ui-icon
            class="icon"
          >
            signal_wifi_statusbar_connected_no_internet_4
          </ui-icon>
          <div class="col">
            <p v-if="loading">{{ titleLoading || title }}</p>
            <p v-if="!loading">{{ title }}</p>
            <sub>{{ subtitle }}</sub>
          </div>
        </div>
      </template>
      <span class="mono">
        {{ detail }}
      </span>
    </ui-collapse>

    <ui-button
      class="button"
      @click="emit('click')"
    >
      <ui-spinner
        v-if="loading"
        active
        size="small"
      >
      </ui-spinner>
      <span v-else>
        Retry
      </span>
    </ui-button>
  </div>
</template>

<script setup>

const {
  title,
  titleLoading,
  subtitle,
  detail,
  loading,
} = defineProps({
  title: {
    type: String,
    default: "Connection error",
  },
  titleLoading: {
    type: String,
    default: "",
  },
  subtitle: {
    type: String,
    default: "",
  },
  detail: {
    type: Object,
    default: "",
  },
  loading: {
    type: Boolean,
    default: false,
  },
});

const emit = defineEmits(["click"]);

</script>

<style scoped>

.error-bar {
  --margin: 14px;
  position: fixed;
  bottom: 0px;
  margin: var(--margin);
  width: calc(100% - var(--margin) * 2);
  max-width: 400px;
  box-sizing: border-box;
  background-color: var(--mdc-theme-error);
  color: var(--mdc-theme-on-error);
  display: flex;
  flex-direction: row;
  align-items: center;
}

.icon {
  margin: 0 12px 0 8px;
}

.button {
  margin: 0 8px 0 12px;
}

.error-bar :deep(.mdc-button) {
  --mdc-theme-primary: var(--mdc-theme-on-error);
  --mdc-theme-on-primary: var(--mdc-theme-error);
}

.collapse {
  flex-grow: 1;
}

.collapse :deep(.mdc-collapse__header) {
  width: 100%;
  padding: 10px;
  box-sizing: border-box;
}

.collapse :deep(.mdc-collapse__content) {
  padding: 0 10px 10px 10px;
}

.message {
  display: flex;
  flex-direction: row;
  align-items: center;
}

.message p {
  margin: 0;
}

sub {
  opacity: 0.8;
}

.col {
  display: flex;
  flex-direction: column;
}

.mono {
  font-family: monospace;
}
</style>
