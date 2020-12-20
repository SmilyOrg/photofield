
export const host = "http://localhost:8080";

async function GET(endpoint, def) {
  const response = await fetch(host + endpoint);
  if (!response.ok) {
    if (def !== undefined) {
      return def;
    }
    throw new Error(response);
  }
  return await response.json();
}

export async function getRegions(x, y, w, h, sceneParams) {
  return GET(`/regions?${sceneParams}&x=${x}&y=${y}&w=${w}&h=${h}`);
}

export async function getCollections() {
  return GET(`/collections`);
}