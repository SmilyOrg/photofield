import { computed, ref, watch, watchEffect } from "vue";
import { refDebounced, useElementSize } from '@vueuse/core';
import { useTask, timeout } from "vue-concurrency";
import { useRoute, useRouter } from "vue-router";
import { useApi, useBufferApi } from "./api";
import qs from "qs";
import { debounce } from "throttle-debounce";

export function useScrollbar(scrollbar, sleep) {

  const attached = ref(null);
  const lastScrollTime = ref(0);
  const ratio = ref(0);
  const y = ref(0);
  const yPerSec = ref(0);
  const max = ref(0);
  const lastScrollPixels = ref(null);
  const scrollToRatio = (r) => {
    scrollbar.value.scroll([0, (r * 100) + "%"]);
  }
  const scrollToPixels = (p) => {
    p = Math.floor(p);
    lastScrollPixels.value = p;
    y.value = p;
    scrollbar.value.scroll([0, p + "px"]);
    onScroll();
  }

  const speedReset = debounce(1000, () => {
    yPerSec.value = 0;
  });

  const onScroll = () => {
    if (!scrollbar.value) return;
    
    const scroll = scrollbar.value.scroll();
    let r = scroll.ratio.y;

    if (lastScrollPixels.value !== null) {
      const diff = Math.abs(scroll.position.y - lastScrollPixels.value);
      lastScrollPixels.value = null;
    }

    // Ratio can be outside of range if the range has changed recently
    r = Math.min(1, Math.max(0, r));

    const now = Date.now();
    
    const yd = scroll.position.y - y.value;
    const elapsed = now - lastScrollTime.value;
    yPerSec.value = Math.abs(yd) * 1000 / elapsed;

    ratio.value = r;
    y.value = scroll.position.y;
    max.value = scroll.max.y;

    lastScrollTime.value = now;

    speedReset();
  }

  const onHostSizeChanged = () => {
    scrollToRatio(ratio.value);
  }

  const onContentSizeChanged = () => {
    scrollToRatio(ratio.value);
  }

  watchEffect(() => {
    if (attached.value) {
      attached.value.options({
        callbacks: {
          onScroll: null,
        },
      });
    }

    attached.value = scrollbar.value;
    if (!scrollbar.value) return;
    
    scrollbar.value.options({
      callbacks: {
        onScroll: onScroll,
        onContentSizeChanged: onContentSizeChanged,
        onHostSizeChanged: onHostSizeChanged,
      },
    });
  });

  watchEffect(() => {
    const el = scrollbar.value?.getElements().scrollbarVertical.scrollbar;
    if (sleep.value) {
      if (el) el.style.opacity = 0;
      scrollbar.value?.sleep();
    } else {
      if (el) el.style.opacity = 1;
      scrollbar.value?.update();
    }
  });

  return {
    ratio,
    y,
    yPerSec,
    max,
    scrollToRatio,
    scrollToPixels,
  }

}

export function useRegion({ scene, id }) {
  
  const valid = computed(() => {
    return !!(scene?.value?.id && !scene.value.loading && id.value);
  });

  const { data, isValidating, mutate } = useApi(() => 
    valid.value &&
    `/scenes/${scene.value.id}/regions/${id.value}`
  );

  const region = ref(null);

  watchEffect(() => {
    if (!id.value) {
      region.value = null;
      return;
    }
    if (!valid.value) {
      region.value = {
        id: id.value,
        loading: true,
      }
      return;
    }
    if (isValidating.value) return;
    region.value = data.value;
  })

  return {
    region,
    mutate,
  };
}

export function useRegionsInBounds({ scene, bounds }) {
  const valid = computed(() => {
    return !!(scene?.value?.id && !scene.value.loading && bounds.value);
  });
  const { items } = useApi(() => 
    valid.value &&
    `/scenes/${scene.value.id}/regions?x=${bounds.value.x}&y=${bounds.value.y}&w=${bounds.value.w}&h=${bounds.value.h}`
  );
  return items;
}

export function useNavigation({ index, count, apply }) {
  const seekIndex = ref(null);
  const finalIndex = computed(() => {
    if (seekIndex.value != null) {
      return seekIndex.value;
    }
    if (typeof index.value == "string") {
      const indexInt = parseInt(index.value, 10);
      return isNaN(indexInt) ? 0 : indexInt;
    }
    return index.value;
  });

  watch(index, () => {
    seekIndex.value = null;
  })

  const navigate = (offsetOrRegion, offset) => {
    let nextIndex;
    if (typeof offsetOrRegion == "string") {
      offsetOrRegion = parseInt(offsetOrRegion, 10);
    }
    if (typeof offsetOrRegion == "number") {
      nextIndex = finalIndex.value + offsetOrRegion;
    } else if (typeof offsetOrRegion == "object" && offsetOrRegion.id) {
      nextIndex = offsetOrRegion.id;
      if (typeof offset == "number") {
        nextIndex += offset;
      }
    } else {
      throw new Error("Unsupported parameter: " + offsetOrRegion);
    }
    if (nextIndex <= 0 || nextIndex > count.value) {
      return false;
    }
    seekIndex.value = nextIndex;
    debouncedSeek(nextIndex);
    return true;
  }

  const debouncedSeek = debounce(1000, index => {
    if (seekIndex.value === null) return;
    apply(index);
  });

  return {
    navigate,
    index: finalIndex,
  }
}

export function useSeekableRegion({ scene, collectionId, regionId }) {

  const router = useRouter();
  const route = useRoute();

  const fileCount = computed(() => {
    return scene.value?.file_count || 0;
  });

  const {
    navigate,
    index,
  } = useNavigation({
    index: regionId,
    count: fileCount,
    apply(index) {
      router.push({
        name: "region",
        params: {
          collectionId: collectionId.value,
          regionId: index,
        },
        query: route.query,
      });
    },
  });

  const { region, mutate } = useRegion({
    scene,
    id: index,
  });
  
  const exit = async () => {
    await router.push({
      name: "collection",
      params: {
        collectionId: collectionId.value,
      },
      query: route.query,
    });
  }

  return {
    region,
    navigate,
    exit,
    mutate,
  }
}

export function useViewport(element) {
  const viewport = useElementSize(element);
  return {
    width: refDebounced(viewport.width, 200),
    height: refDebounced(viewport.height, 200),
  }
}

export function useRetry(f) {
  const count = ref(0);
  const delays = [10, 50, 100, 200, 500, 1000];
  const task = useTask(function*() {
    const c = count.value;
    const delay = delays[Math.min(delays.length - 1, c)];
    count.value = c + 1;
    yield timeout(delay);
    f();
  }).keepLatest()
  const run = () => {
    task.perform();
  }
  const reset = () => {
    count.value = 0;
  }
  return {
    run,
    reset,
  }
}

export function useViewDelta(viewHistory, viewport, now) {
  return computed(() => {
    let sumx = 0;
    let sumz = 0;
    const viewportWidth = viewport.width.value;
    let vp = null;
    const n = now.value.getTime();
    const maxt = 250;
    for (const v of viewHistory.value) {
      if (vp && v.snapshot) {
        const dt = vp.timestamp - v.timestamp;
        if (dt < 0.1) continue;
        const et = n - v.timestamp;
        const w = Math.max(0, 1-et/maxt);

        const dx = vp.snapshot.x - v.snapshot.x;
        sumx += (dx*w)/dt;
        
        const dz = viewportWidth / vp.snapshot.w - viewportWidth / v.snapshot.w;
        sumz += (dz*w)/dt;
      }
      vp = v;
    }
    return {
      x: sumx*maxt/viewportWidth,
      zoom: sumz*maxt,
    }
  })
}

export function useContextMenu(menu, viewer, scene) {
  
  const openEvent = ref(false);
  const flip = ref({ x: false, y: false });

  const open = (event) => {
    menu.value?.open(event);
    openEvent.value = event;
  }

  const close = () => {
    if (!openEvent.value) return;
    openEvent.value = null;
    menu.value.close();
  }

  const eventBounds = computed(() => {
    const event = openEvent.value;
    const pos = event && viewer.value?.elementToViewportCoordinates(event);
    if (!pos) return null;
    return {
      x: pos.x,
      y: pos.y,
      w: 0,
      h: 0,
    }
  })

  const regions = useRegionsInBounds({ scene, bounds: eventBounds });
  const region = computed(() => {
    return regions.value && regions.value.length >= 1 && regions.value[0];
  })

  const onContextMenu = (event) => {
    if (!menu.value) return;
    open(event);
    const menuWidth = 250;
    const menuHeight = 300;
    const right = event.x + menuWidth;
    const bottom = event.y + menuHeight;
    flip.value = {
      x: right > window.innerWidth,
      y: bottom > window.innerHeight,
    }
  }
  
  return {
    onContextMenu,
    flip,
    openEvent,
    close,
    region,
  }
}

export function useTimeline({ scene, viewport, scrollRatio }) {
  
  const {
    data: datesBuffer,
  } = useBufferApi(() => 
    scene?.value?.id &&
    !scene?.value?.loading &&
    viewport?.height.value &&
    `/scenes/${scene.value.id}/dates?${qs.stringify({
      height: viewport.height.value,
    })}`
  )

  const timestamps = computed(() => {
    return new Uint32Array(datesBuffer.value);
  });

  const date = computed(() => {
    if (!timestamps.value || timestamps.value.length < 1) return;
    const index =
      Math.min(timestamps.value.length - 1,
      Math.max(0,
      Math.floor(
        scrollRatio.value * (timestamps.value.length - 1)
      )));
    const timestamp = timestamps.value[index];
    return new Date(timestamp * 1000);
  })

  return {
    date,
  }
}
