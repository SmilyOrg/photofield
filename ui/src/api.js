import useSWRV from "swrv";
import { computed, watch, ref } from "vue";
import qs from "qs";

const host = import.meta.env.VITE_API_HOST || "/api";

async function fetcher(endpoint) {
  const response = await fetch(host + endpoint);
  if (!response.ok) {
    console.error(response);
    throw new Error(response.statusText);
  }
  return await response.json();
}

export async function get(endpoint, def) {
  const response = await fetch(host + endpoint);
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
  const response = await fetch(host + endpoint, {
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

export async function getRegion(id, sceneParams) {
  return get(`/regions/${id}?${sceneParams}`);
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

export function getTileUrl(sceneId, level, x, y, tileSize, debug) {
  const params = {
    tile_size: tileSize,
    zoom: level,
    x,
    y,
  };
  for (const key in debug) {
    if (Object.hasOwnProperty.call(debug, key)) {
      if (debug[key]) params[`debug_${key}`] = debug[key];
    }
  }
  let url = `${host}/scenes/${sceneId}/tiles?${qs.stringify(params)}`;
  return url;
}

export function getFileUrl(id, filename) {
  if (!filename) {
    return `${host}/files/${id}`;
  }
  return `${host}/files/${id}/original/${filename}`;
}

export async function getFileBlob(id) {
  return getBlob(`/files/` + id);
}

export function getThumbnailUrl(id, size, filename) {
  return `${host}/files/${id}/variants/${size}/${filename}`;
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
  return {
    ...response,
    items,
    itemsMutate,
  }
}

async function bufferFetcher(endpoint) {
  const response = await fetch(host + endpoint);
  if (!response.ok) {
    console.error(response);
    throw new Error(response.statusText);
  }
  return await response.arrayBuffer();
}

export function useBufferApi(getUrl, config) {
  return useSWRV(getUrl, bufferFetcher, config);
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
