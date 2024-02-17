import useSWRV from "swrv";
import { computed, watch, ref } from "vue";
import qs from "qs";
import { useRetry } from "./use";

let _host = null;
function host() {
  if (_host) {
    return _host;
  }
  const cookieHost = getCookie("photofield-api-host");
  if (cookieHost) {
    _host = cookieHost;
    return _host;
  }
  _host = import.meta.env.VITE_API_HOST || "/api";
  return _host;
}

function getCookie(name) {
  const cookies = document.cookie.split("; ");
  for (let i = 0; i < cookies.length; i++) {
    const cookie = cookies[i].split("=");
    if (cookie[0] === name) {
      return cookie[1];
    }
  }
  return null;
}

async function fetcher(endpoint) {
  const response = await fetch(host() + endpoint);
  if (!response.ok) {
    console.error(response);
    throw new Error(response.statusText);
  }
  return await response.json();
}

export async function get(endpoint, def) {
  const response = await fetch(host() + endpoint);
  if (!response.ok) {
    if (def !== undefined) {
      return def;
    }
    console.error(response);
    throw new Error(response.statusText);
  }
  return await response.json();
}

export async function post(endpoint, body, def) {
  const response = await fetch(host() + endpoint, {
    method: "POST",
    body: JSON.stringify(body),
    headers: {
      "Content-Type": "application/json; charset=utf-8",
    }
  });
  if (!response.ok) {
    if (def !== undefined) {
      return def;
    }
    console.error(response);
    throw new Error(response.statusText);
  }
  return await response.json();
}

export async function getRegions(sceneId, x, y, w, h) {
  if (!sceneId) return null;
  const response = await get(`/scenes/${sceneId}/regions?x=${x}&y=${y}&w=${w}&h=${h}`);
  return response.items;
}

export async function getRegionsWithFileId(sceneId, id) {
  if (!sceneId) return null;
  const response = await get(`/scenes/${sceneId}/regions?file_id=${id}`);
  return response.items;
}

export async function getRegion(sceneId, id) {
  return get(`/scenes/${sceneId}/regions/${id}`);
}

export async function getCenterRegion(sceneId, x, y, w, h) {
  const regions = await getRegions(sceneId, x, y, w, h);
  if (!regions) return null;
  const cx = x + w*0.5;
  const cy = y + h*0.5;
  let minDistSq = Infinity;
  let minRegion = null;
  for (let i = 0; i < regions.length; i++) {
    const region = regions[i];
    const rcx = region.bounds.x + region.bounds.w*0.5;
    const rcy = region.bounds.y + region.bounds.h*0.5;
    const dx = rcx - cx;
    const dy = rcy - cy;
    const distSq = dx*dx + dy*dy;
    if (distSq < minDistSq) {
      minDistSq = distSq;
      minRegion = region;
    }
  }
  return minRegion;
}

export async function getCollections() {
  return get(`/collections`);
}

export async function getCollection(id) {
  return get(`/collections/` + id);
}

export async function createTask(type, id) {
  return await post(`/tasks`, {
    type,
    collection_id: id
  });
}

export function getTileUrl(sceneId, level, x, y, tileSize, backgroundColor, extraParams) {
  const params = {
    tile_size: tileSize,
    zoom: level,
    x,
    y,
    ...extraParams,
  };
  if (backgroundColor) {
    params.background_color = backgroundColor;
  }
  let url = `${host()}/scenes/${sceneId}/tiles?${qs.stringify(params, { arrayFormat: "comma" })}`;
  return url;
}

export function getFileUrl(id, filename) {
  if (!filename) {
    return `${host()}/files/${id}`;
  }
  return `${host()}/files/${id}/original/${filename}`;
}

export async function getFileBlob(id) {
  return getBlob(`/files/` + id);
}

export function getThumbnailUrl(id, size, filename) {
  return `${host()}/files/${id}/variants/${size}/${filename}`;
}

export function useApi(getUrl, config) {
  const response = useSWRV(getUrl, fetcher, config);
  const items = computed(() => response.data.value?.items);
  const itemsMutate = async getItems => {
    if (!getItems) {
      await response.mutate();
      return;
    } 
    const items = await getItems();
    await response.mutate(() => ({
      items,
    }));
  };
  const errorTime = computed(() => {
    if (response.error.value) {
      return new Date();
    }
    return null;
  });
  return {
    ...response,
    items,
    itemsMutate,
    errorTime,
  }
}

export function useScene({
  collectionId,
  layout,
  sort,
  imageHeight,
  viewport,
  search,
}) {
  
  const sceneParams = computed(() =>
    viewport?.width?.value &&
    viewport?.height?.value &&
    {
      layout: layout.value,
      sort: sort?.value || undefined,
      image_height: imageHeight?.value || undefined,
      collection_id: collectionId.value,
      viewport_width: viewport.width.value,
      viewport_height: viewport.height.value,
      search: search?.value || undefined,
    }
  );

  const {
    items: scenes,
    isValidating: scenesLoading,
    itemsMutate: scenesMutate,
  } = useApi(() => sceneParams.value && `/scenes?` + qs.stringify(sceneParams.value));

  const scene = computed(() => {
    const list = scenes?.value;
    if (!list || list.length == 0) return null;
    return list[0];
  });

  const recreateScenesInProgress = ref(0);
  const recreateScene = async () => {
    recreateScenesInProgress.value = recreateScenesInProgress.value + 1;
    const params = sceneParams.value;
    await scenesMutate(async () => ([await createScene(params)]));
    recreateScenesInProgress.value = recreateScenesInProgress.value - 1;
  }

  watch(scenes, async scenes => {
    if (!scenes || scenes.length === 0) {
      console.log("scene not found, creating...");
      await recreateScene();
      return;
    }
  })

  const { run, reset } = useRetry(scenesMutate);

  const loadSpeed = ref(0);

  watch(scene, async (newValue, oldValue) => {
    if (newValue?.loading) {
      let prev =
        oldValue?.load_count !== undefined ?
        oldValue?.load_count :
        oldValue?.file_count || 0;
      let next =
        newValue.load_count !== undefined ?
        newValue.load_count :
        newValue.file_count
      if (prev > next) {
        prev = 0;
      }
      loadSpeed.value = next - prev;
      run();
      return;
    }
    reset();
    loadSpeed.value = 0;
    
    if (newValue?.stale && !newValue?.loading && !oldValue?.loading) {
      console.log("scene stale, recreating...");
      await recreateScene();
      return;
    }
  })

  return {
    scene,
    recreate: recreateScene,
    loading: scenesLoading,
    loadSpeed,
  }
}

async function bufferFetcher(endpoint) {
  const response = await fetch(host() + endpoint);
  if (!response.ok) {
    console.error(response);
    throw new Error(response.statusText);
  }
  return await response.arrayBuffer();
}

export function useBufferApi(getUrl, config) {
  return useSWRV(getUrl, bufferFetcher, config);
}

async function textFetcher(endpoint) {
  const response = await fetch(host() + endpoint);
  if (!response.ok) {
    console.error(response);
    throw new Error(response.statusText);
  }
  return await response.text();
}

export function useTextApi(getUrl, config) {
  return useSWRV(getUrl, textFetcher, config);
}

export function useTasks() {
  const intervalMs = 250;
  const response = useApi(
    () => `/tasks`
  );
  const { items, mutate } = response;
  const timer = ref(null);
  const resolves = ref([]);
  const updateUntilDone = async () => {
    await mutate();
    if (resolves.value) {
      return new Promise(resolve => resolves.value.push(resolve));
    }
    return;
  }
  watch(items, items => {
    if (items.length > 0) {
      if (!timer.value) {
        timer.value = setTimeout(() => {
          timer.value = null;
          mutate();
        }, intervalMs);
      }
    } else {
      resolves.value.forEach(resolve => resolve());
      resolves.value.length = 0;
    }
  })
  return {
    ...response,
    updateUntilDone,
  };
}

export async function createScene(params) {
  return await post(`/scenes`, params);
}

export async function addTag(body) {
  return await post(`/tags`, body);
}

export async function postTagFiles(id, body) {
  return await post(`/tags/${id}/files`, body);
}
