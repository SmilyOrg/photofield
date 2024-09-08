<template>
  <div class="spinner" :class="{ hidden: !loading || !visible, removed: !visible }">
    <div class="label">
      <template v-if="loading && total == 0">Loading</template>
      <template v-else-if="loading">{{ (interpolatedTotal && Math.round(interpolatedTotal).toLocaleString()) || 0 }} {{unit || 'files'}}</template>
      <template v-else>{{ total && total.toLocaleString() }} files</template>
    </div>
    <svg :viewBox="`0 0 ${width} ${height}`" xmlns="http://www.w3.org/2000/svg">
      <rect
        v-for="r in rects"
        :key="r.id"
        ref="rects"
        :x="r.x"
        :y="y + r.y + (1 - easeOutExpo(Math.min(1, (now - r.t)/1000)) * 100)"
        :opacity="easeOutExpo((now - r.t)/1000 * 2)"
        :width="r.w"
        :height="r.h"
        fill="#999"
      ></rect>
    </svg>
  </div>
</template>

<script>
export default {

  props: {
    total: Number,
    speed: Number,
    divider: Number,
    loading: Boolean,
    unit: String,
  },

  data() {
    return {
      startDelay: 500,
      finishRemoveDelay: 2000,
      smoothSpeedEasing: 0.95,
      aspectRatios: [
        { weight: 10, ratio: 4/3 },
        { weight: 4, ratio: 3/4 },
        { weight: 8, ratio: 10/16 },
      ],
      rowHeight: 180,
      spacing: 10,
      newRowHeightThreshold: 0.8,
      minRowItems: 3,

      minSpeed: 0.05,
      maxSpeed: 2,
      width: 1000,
      height: 1000,

      poolSize: 60,

      interpolatedTotal: 0,
      smoothSpeed: 0,
      y: 0,
      now: 0,
      rects: [],
      startTime: 0,
      finishTime: 0,
      running: false,
      visible: false,
    }
  },

  mounted() {
    this.setupAspectRatios();

    this.lastTotal = 0;
    this.lastTotalTime = Date.now();

    for (let i = 0; i < this.poolSize; i++) {
      this.rects.push({ id: `r${i}`, x: 0, y: -1, w: 0.1, h: 0.1, t: 0 });
    }

    this.row = { rects: this.rects.slice(0, 10), x: 0, y: 60, spacing: this.spacing };
  },

  unmounted() {
    this.running = false;
  },

  watch: {
    loading: {
      immediate: true,
      handler(newValue, oldValue) {
        if (newValue == oldValue) return;
        const now = Date.now();
        if (newValue) {
          if (this.running) return;
          this.startTime = now;
          this.last = now;
          this.running = true;
          requestAnimationFrame(this.frame);
        } else {
          this.finishTime = now;
        }
      },
    },
    total: {
      immediate: true,
      handler(newValue, oldValue) {
        this.lastTotal = oldValue < newValue ? oldValue : newValue;
        this.lastTotalTime = Date.now();
      },
    },
  },

  methods: {
    easeOutExpo(x) {
      return x === 1 ? 1 : 1 - Math.pow(2, -10 * x);
    },
    setupAspectRatios() {
      let totalWeight = this.aspectRatios.reduce((total, a) => total + a.weight, 0);
      let sum = 0;
      this.aspectRatios.forEach(a => {
        sum += a.weight;
        a.cumulativeWeight = sum / totalWeight;
      });
    },
    getRandomSize() {
      const pick = Math.random();
      for (let i = 0; i < this.aspectRatios.length; i++) {
        const a = this.aspectRatios[i];
        if (a.cumulativeWeight >= pick || i == this.aspectRatios.length - 1) {
          const h = 100;
          const w = h * a.ratio;
          return { w, h };
        }
      }
    },
    addRow({ rects, x, y, spacing }) {
      const boundsWidth = this.width;
      let remaining = [];
      const now = Date.now();
      for (let i = 0; i < rects.length; i++) {
        const r = rects[i];
        r.x = x;
        r.y = y;
        r.t = now + 200 + Math.random() * 100;

        const size = this.getRandomSize();
        r.w = size.w;
        r.h = size.h;
        
        const scale = this.rowHeight / r.h;
        r.w *= scale;
        r.h *= scale;
        
        x += r.w + spacing;
        if (x >= boundsWidth) {
          remaining = rects.slice(i + 1);
          rects = rects.slice(0, i + 1);
          break;
        }
      }
      const rowWidth = x - spacing;
      const totalSpacing = (rects.length - 1) * spacing; 
      const scale = (boundsWidth - totalSpacing) / (rowWidth - totalSpacing);
      x = 0;
      for (let i = 0; i < rects.length; i++) {
        const r = rects[i];
        r.x = x;
        r.w *= scale;
        r.h *= scale;
        x += r.w + spacing;
      }
      x = 0;
      y += this.rowHeight * scale + spacing;
      return { rects: remaining, x, y, spacing };
    },
    frame() {
      const now = Date.now();
      this.now = now;

      if (now < this.startTime + this.startDelay) {
        if (!this.loading) this.running = false;
        if (this.running) requestAnimationFrame(this.frame);
        return;
      }
      this.visible = true;

      const dt = (now - this.last)/1000;
      const rects = this.rects;

      this.interpolatedTotal = this.lastTotal + (this.total - this.lastTotal) * Math.min(1, (now - this.lastTotalTime) / 1000);

      const boundedSpeed = this.speed == 0 ? 0 : Math.min(this.maxSpeed, Math.max(this.minSpeed, this.speed / this.divider));
      this.smoothSpeed += (boundedSpeed - this.smoothSpeed) * this.smoothSpeedEasing * dt;
      if (!this.loading) {
        this.smoothSpeed *= 0.9;
      }

      this.y += -this.smoothSpeed * this.height * dt;

      const outOfBounds = [];
      for (let i = 0; i < rects.length; i++) {
        const r = rects[i];
        const bottom = this.y + r.y + r.h;
        if (bottom < 0) {
          outOfBounds.push(r);
        }
      }
      const lowest = this.y + this.row.y;

      if (this.loading && lowest < this.height * this.newRowHeightThreshold) {
        this.row.rects = outOfBounds;
        this.row = this.addRow(this.row);
      }

      if (!this.loading && (now - this.finishTime) > this.finishRemoveDelay) {
        this.running = false;
        this.visible = false;
      }

      this.last = now;
      if (this.running) requestAnimationFrame(this.frame);
    }
  }

};
</script>

<style scoped>

.spinner {
  background: white;
  box-sizing: border-box;
  padding: 12px;
  border-radius: 5px;
  transition: opacity 1s cubic-bezier(1,0,.86,0);
}

.hidden {
  opacity: 0
}

.removed {
  display: none;
}

.label {
  width: 100%;
  text-align: center;
  margin-bottom: 8px;
}

</style>
