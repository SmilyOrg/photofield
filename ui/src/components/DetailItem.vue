<template>
  <div class="detail-item">
    <dt>
      <ui-skeleton
        :loading="loading"
        :active="true"
        class="icon"
        :avatar="{ size: 24 }"
        :title="false"
        :paragraph="false"
      >
        <ui-icon class="icon" :outlined="outlined">{{ icon }}</ui-icon>
      </ui-skeleton>
    </dt>
    <dd class="content">
      <ui-skeleton
        :loading="loading"
        :active="true"
        :title="false"
        :paragraph="{ rows: 2 }"
      >
        <span :class="$tt('subtitle1')">{{ title }}</span>
        <div>
          <span
            v-for="sub in subtitles"
            :key="sub"
            :class="[$tt('subtitle2'), 'sub']"
          >
            {{ sub }}
          </span>
        </div>
      </ui-skeleton>
    </dd>
  </div>
</template>

<script setup>

const props = defineProps({
    icon: String,
    title: String,
    subtitles: Array,
    outlined: Boolean,
    loading: Boolean,
});

</script>

<style scoped>

.detail-item {
  display: flex;
  padding: 18px 24px ;
  align-items: top;
}

.icon {
  margin-top: 1px;
  width: 24px;
  height: 24px;
  color: var(--mdc-theme-text-primary-on-background);
}

.content {
  display: flex;
  flex-direction: column;
  margin-left: 16px;
  width: 100%;
}

.sub {
  font-weight: 400;
  margin-right: 12px;
  color: var(--mdc-theme-text-secondary-on-background);
}

</style>