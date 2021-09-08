import useSWRV from "swrv";
import { computed } from "vue";
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

export async function reindexCollection(id) {
  return await post(`/index-tasks`, {
    collection_id: id
  });
}

export function getTileUrl(sceneId, level, x, y, tileSize) {
  let url = `${host}/scenes/${sceneId}/tiles?${qs.stringify({
    tile_size: tileSize,
    zoom: level,
    x,
    y,
  })}`;
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
  return `${host}/files/${id}/image-variants/${size}/${filename}`;
}

export function getVideoUrl(id, size, filename) {
  return `${host}/files/${id}/video-variants/${size}/${filename}`;
}

export function useApi(getUrl) {
  const response = useSWRV(getUrl, fetcher);
  const items = computed(() => response.data.value?.items);
  const itemsMutate = async getItems => {
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

export async function createScene(params) {
  return await post(`/scenes`, params);
}
