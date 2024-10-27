<template>
  <div
    class="date-strip"
    v-if="date"
  >
    <div class="year">{{ dateFormat(date, "yyyy") }}</div>
    <div class="month">{{ dateFormat(date, "MMM") }}</div>
    <div class="day">{{ dateFormat(date, "d") }}</div>
    <div class="weekday">{{ dateFormat(date, "EEE") }}</div>
    <div class="time">{{ dateFormat(date, "HH:mm") }}</div>
  </div>
    
</template>

<script>
import dateFormat from 'date-fns/format';

export default {
  props: ["date"],
  methods: {
    dateFormat,
  }
};

</script>

<style scoped>

.date-strip {
  background: var(--mdc-theme-background);
  color: var(--mdc-theme-text-primary-on-background);
  padding: 10px;
  text-align: center;
}

.month {
  font-size: 1.4em;
}

.day {
  font-size: 2em;
}

.year, .weekday {
  font-size: 0.9em;
  color: var(--mdc-theme-text-secondary-on-background);
}

.time {
  font-size: 1em;
  margin-top: 12px;
  letter-spacing: 1px;
  color: var(--mdc-theme-text-secondary-on-background);
}

</style>