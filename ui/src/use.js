import { computed, ref, watch } from "vue";
import { refDebounced, useElementSize, useTimeout } from '@vueuse/core';
import { useTask, timeout } from "vue-concurrency";
import { useRoute, useRouter } from "vue-router";
import { addTag, postTagFiles, useApi, useBufferApi } from "./api";
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
    if (scroll.max.y === 0) return;

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
    onScroll();
  }

  watch([scrollbar, attached], () => {
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

  watch([scrollbar, sleep], () => {
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

  const region = computed(() => {
    if (!id.value) {
      return null;
    }
    if (!valid.value) {
      return {
        id: id.value,
        loading: true,
      };
    }
    if (isValidating.value) {
      return {
        id: id.value,
        loading: true,
      };
    }
    return data.value;
  });

  return {
    region,
    loading: isValidating,
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

// Configurable range size
const DEFAULT_RANGE_SIZE = 100;

function useRangeUrl({ sceneId, regionId, rangeSize = DEFAULT_RANGE_SIZE, offset = 0 }) {
  return computed(() => {
    if (!sceneId.value || !regionId.value) return null;
    
    const id = regionId.value;
    const rangeIndex = Math.floor((id - 1) / rangeSize) + offset;
    
    if (rangeIndex < 0) return null;
    
    const rangeStart = rangeIndex * rangeSize + 1;
    const rangeEnd = rangeStart + rangeSize - 1;
    
    return `/scenes/${sceneId.value}/regions?id_range=${rangeStart}:${rangeEnd}&fields=(id,bounds)`;
  });
}

export function useSeekableRegion({ scene, collectionId, regionId, rangeSize = DEFAULT_RANGE_SIZE }) {
  const router = useRouter();
  const route = useRoute();

  const fileCount = computed(() => scene.value?.file_count || 0);

  // Parse regionId to number for internal use
  const currentRegionId = computed(() => {
    if (typeof regionId.value === "string") {
      const parsed = parseInt(regionId.value, 10);
      return isNaN(parsed) ? null : parsed;
    }
    return regionId.value;
  });

  // Navigation state
  const seekingId = ref(null);
  const activeRegionId = computed(() => 
    seekingId.value !== null ? seekingId.value : currentRegionId.value
  );

  // Reactive URLs for current, previous, and next ranges
  const currentRangeUrl = useRangeUrl({ 
    sceneId: computed(() => scene.value?.id), 
    regionId: activeRegionId, 
    rangeSize 
  });
  const previousRangeUrl = useRangeUrl({ 
    sceneId: computed(() => scene.value?.id), 
    regionId: activeRegionId, 
    rangeSize, 
    offset: -1 
  });
  const nextRangeUrl = useRangeUrl({ 
    sceneId: computed(() => scene.value?.id), 
    regionId: activeRegionId, 
    rangeSize, 
    offset: 1 
  });

  // Declarative API calls for current, previous, and next ranges
  const currentRange = useApi(currentRangeUrl);
  
  // Prefetch previous and next ranges
  useApi(previousRangeUrl);
  useApi(nextRangeUrl);

  // Full region API for upgrading (debounced)
  const debouncedActiveRegionId = refDebounced(activeRegionId, 200);
  const { data: fullRegionData, mutate } = useApi(() => 
    scene.value?.id && debouncedActiveRegionId.value &&
    `/scenes/${scene.value.id}/regions/${debouncedActiveRegionId.value}`
  );

  // Computed region that prioritizes full data over range data
  const region = computed(() => {
    const id = activeRegionId.value;
    if (!id) return null;

    // Prefer full region data if available and IDs match
    if (fullRegionData.value && fullRegionData.value.id === id) {
      return { ...fullRegionData.value, minimal: false };
    }

    // Fallback to range data
    const rangeRegion = currentRange.data.value?.items?.find(item => item.id === id);
    if (rangeRegion) {
      return { ...rangeRegion, minimal: true };
    }

    // Loading state
    return { id, loading: true };
  });

  const { ready: notSeeking, start: setSeeking } = useTimeout(200, { controls: true });

  // Navigation function
  const navigate = (offsetOrRegion, offset) => {
    setSeeking();
    
    let nextId;
    if (typeof offsetOrRegion === "string") {
      offsetOrRegion = parseInt(offsetOrRegion, 10);
    }
    if (typeof offsetOrRegion === "number") {
      nextId = activeRegionId.value + offsetOrRegion;
    } else if (typeof offsetOrRegion === "object" && offsetOrRegion.id) {
      nextId = offsetOrRegion.id;
      if (typeof offset === "number") {
        nextId += offset;
      }
    } else {
      throw new Error("Unsupported parameter: " + offsetOrRegion);
    }
    
    if (nextId <= 0 || nextId > fileCount.value) {
      return false;
    }
    
    seekingId.value = nextId;
    return true;
  };

  // Reset seeking state when URL catches up
  watch(regionId, (newRegionId) => {
    if (seekingId.value !== null && 
        parseInt(seekingId.value, 10) === parseInt(newRegionId, 10)) {
      seekingId.value = null;
    }
  });

  const exit = async () => {
    await router.push({
      name: "collection",
      params: {
        collectionId: collectionId.value,
      },
      query: route.query,
    });
  };

  return {
    region,
    navigate,
    exit,
    mutate,
    isSeeking: computed(() => !notSeeking.value),
    isUpgrading: computed(() => 
      region.value?.minimal && !!fullRegionData.value && 
      fullRegionData.value.id === activeRegionId.value
    ),
  };
}

export function useViewport(element) {
  const viewport = useElementSize(element);
  return {
    width: refDebounced(viewport.width, 500),
    height: refDebounced(viewport.height, 500),
  }
}

export function useRegionZoom({ view, region }) {
  return computed(() => {
    const viewBounds = view?.value;
    const regionBounds = region?.value?.bounds;
    if (!viewBounds || !regionBounds) return 1;
    const zoomX = regionBounds.w / viewBounds.w;
    const zoomY = regionBounds.h / viewBounds.h;
    return Math.max(zoomX, zoomY);
  });
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
  
  const openEvent = ref(null);
  const flip = ref({ x: false, y: false });

  const open = (event) => {
    openEvent.value = event;
  }

  const close = () => {
    if (!openEvent.value) return;
    openEvent.value = null;
  }

  watch([openEvent, menu], async ([event]) => {
    if (!event) return;
    const el = menu?.value?.$el;
    if (!el) return;
    const menuWidth = el.offsetWidth;
    const menuHeight = el.offsetHeight;
    let x = (event.clientX + menuWidth > window.innerWidth) ? event.clientX - menuWidth : event.clientX;
    let y = (event.clientY + menuHeight > window.innerHeight) ? event.clientY - menuHeight : event.clientY;
    if (x < 0) x = 0;
    if (y < 64) y = 64;
    el.style.left = x + "px";
    el.style.top = y + "px";
  });

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
    if (!openEvent.value) return null;
    return regions.value && regions.value.length >= 1 && regions.value[0];
  })

  const onContextMenu = (event) => {
    open(event);
  }
  
  return {
    onContextMenu,
    // flip,
    openEvent,
    close,
    region,
  }
}

export function useTimestamps({ scene, height }) {
  const {
    data: datesBuffer,
  } = useBufferApi(() => 
    scene?.value?.id &&
    !scene?.value?.loading &&
    height.value &&
    `/scenes/${scene.value.id}/dates?${qs.stringify({
      height: Math.round(height.value),
    })}`
  )

  const timestamps = computed(() => {
    return new Uint32Array(datesBuffer.value);
  });

  return timestamps;
}

export function useTimestampsDate({ timestamps, ratio }) {
  return computed(() => {
    if (!timestamps.value || timestamps.value.length < 1) return null;
    const index =
      Math.min(timestamps.value.length - 1,
        Math.max(0,
          Math.round(
            ratio.value * (timestamps.value.length - 1)
          )
        )
      );
    const timestamp = timestamps.value[index];
    const now = new Date();
    const offset = now.getTimezoneOffset()*60;
    const d = new Date((timestamp + offset) * 1000);
    if (isNaN(Number(d))) return null;
    return d;
  })
}

export function useTags({ supported, selectTag, collectionId, scene }) {
  const selectBounds = async (op, bounds) => {
    if (!supported.value) return;
    let id = selectTag.value?.id;
    if (!id) {
      const tag = await addTag({
        selection: true,
        collection_id: collectionId.value,
      });
      id = tag.id;
    }
    return await postTagFiles(id, {
      op,
      scene_id: scene.value.id,
      bounds
    });
  };

  return {
    selectBounds
  };
};

export function useRegionTags({ region, updateRegion }) {
  const tags = computed(() => {
    return region.value?.data?.tags || [];
  });

  const fileId = computed(() => region.value?.data?.id);

  const op = async (tag, op) => {
    const id = tag?.id || tag || null;
    if (!fileId.value || !id) {
      return;
    }
    await postTagFiles(id, {
      op,
      file_id: fileId.value,
    });
    if (updateRegion) await updateRegion();
  }

  return {
    tags,
    add: tag => op(tag, "ADD"),
    remove: tag => op(tag, "SUBTRACT"),
    invert: tag => op(tag, "INVERT"),
  };
}
