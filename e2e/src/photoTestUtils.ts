/**
 * Utility functions for testing photo interactions in Photofield
 */

import { Page } from '@playwright/test';

// Extend the Window interface to include our global photofield object
declare global {
  interface Window {
    __PHOTOFIELD__?: {
      currentScene?: Scene;
    };
  }
}

interface Scene {
  id: string | null;
  bounds?: { x: number; y: number; w: number; h: number };
  file_count?: number;
  loading?: boolean;
}

export interface PhotoRegion {
  id: number;
  bounds: {
    x: number;
    y: number;
    w: number;
    h: number;
  };
  data?: {
    id: number;
    filename: string;
    path: string;
    width: number;
    height: number;
  };
}

export interface SceneCoordinates {
  x: number;
  y: number;
}

export interface PixelCoordinates {
  x: number;
  y: number;
}

/**
 * Get the API host from the page context
 */
export async function getApiHost(page: Page): Promise<string> {
  return await page.evaluate(() => {
    // Try to get from localStorage first (development)
    const stored = localStorage.getItem('photofield-api-host');
    if (stored) return stored;
    
    // Try to get from cookie (production)
    const cookies = document.cookie.split(';');
    for (const cookie of cookies) {
      const [name, value] = cookie.trim().split('=');
      if (name === 'photofield-api-host') {
        return decodeURIComponent(value);
      }
    }
    
    // Fallback to current origin
    return window.location.origin;
  });
}

/**
 * Get the current scene ID from the page
 */
export async function getSceneId(page: Page): Promise<string | null> {
  return await page.evaluate(() => {
    // Try to extract from URL
    const pathMatch = window.location.pathname.match(/\/collections\/([^\/]+)/);
    if (pathMatch) {
      // For collection URLs, we need to make an API call to get scene
      return null; // Will need to be handled by caller
    }
    
    // Try to get from Vue router if available
    try {
      const app = (window as any).__VUE_APP__;
      if (app?.config?.globalProperties?.$route) {
        return app.config.globalProperties.$route.params.sceneId;
      }
    } catch (e) {
      // Vue app not available or different structure
    }
    
    return null;
  });
}

/**
 * Get photo regions from the API
 */
export async function getPhotoRegions(
  page: Page, 
  sceneId?: string,
  bounds?: { x: number; y: number; w: number; h: number },
  limit?: number
): Promise<PhotoRegion[]> {
  // Get scene ID if not provided
  const actualSceneId = sceneId || await getCurrentSceneId(page);
  if (!actualSceneId) {
    throw new Error('No scene ID available');
  }
  
  const apiHost = await getApiHost(page);
  
  let url = `${apiHost}/scenes/${actualSceneId}/regions`;
  const params = new URLSearchParams();
  
  if (bounds) {
    params.set('x', bounds.x.toString());
    params.set('y', bounds.y.toString());
    params.set('w', bounds.w.toString());
    params.set('h', bounds.h.toString());
  }
  
  if (limit) {
    params.set('limit', limit.toString());
  }
  
  if (params.toString()) {
    url += '?' + params.toString();
  }
  
  const response = await page.evaluate(async (url) => {
    const res = await fetch(url);
    return res.json();
  }, url);
  
  return response.items || [];
}

/**
 * Get the closest photo to specific coordinates
 */
export async function getClosestPhoto(
  page: Page,
  x: number,
  y: number,
  sceneId?: string
): Promise<PhotoRegion | null> {
  // Get scene ID if not provided
  const actualSceneId = sceneId || await getCurrentSceneId(page);
  if (!actualSceneId) {
    throw new Error('No scene ID available');
  }
  
  const apiHost = await getApiHost(page);
  const url = `${apiHost}/scenes/${actualSceneId}/regions?x=${x}&y=${y}&closest=true&limit=1`;
  
  const response = await page.evaluate(async (url) => {
    const res = await fetch(url);
    return res.json();
  }, url);
  
  return response.items?.[0] || null;
}

/**
 * Convert scene coordinates to pixel coordinates for clicking
 */
export async function sceneToPixelCoordinates(
  page: Page,
  sceneX: number,
  sceneY: number
): Promise<PixelCoordinates | null> {
  return await page.evaluate(({ sceneX, sceneY }) => {
    const tileViewer = document.querySelector('.tileViewer');
    if (!tileViewer) return null;
    
    // Try to access the OpenLayers map instance
    const mapCanvas = tileViewer.querySelector('canvas');
    if (!mapCanvas) return null;
    
    // Get the bounding box of the canvas
    const rect = mapCanvas.getBoundingClientRect();

    return { x: rect.x + sceneX, y: rect.y + sceneY };
  }, { sceneX, sceneY });
}

/**
 * Click on a photo by its scene coordinates
 */
export async function clickPhotoAtCoordinates(
  page: Page,
  sceneX: number,
  sceneY: number
): Promise<void> {
  const pixelCoords = await sceneToPixelCoordinates(page, sceneX, sceneY);
  if (!pixelCoords) {
    throw new Error('Could not convert scene coordinates to pixel coordinates');
  }
  
  await page.mouse.click(pixelCoords.x, pixelCoords.y);
}

/**
 * Get coordinates for a photo region (center point) using direct API call
 */
export async function getRegionCenter(
  page: Page,
  regionId: number,
  sceneId?: string
): Promise<SceneCoordinates | null> {
  const actualSceneId = sceneId || await getCurrentSceneId(page);
  if (!actualSceneId) {
    throw new Error('No scene ID available');
  }

  const apiHost = await getApiHost(page);
  const url = `${apiHost}/scenes/${actualSceneId}/regions/${regionId}`;
  
  try {
    const region = await page.evaluate(async (url) => {
      const res = await fetch(url);
      if (!res.ok) return null;
      return res.json();
    }, url);

    if (!region || !region.bounds) {
      return null;
    }

    return {
      x: region.bounds.x + region.bounds.w / 2,
      y: region.bounds.y + region.bounds.h / 2
    };
  } catch (error) {
    return null;
  }
}

/**
 * Click on the first photo in the scene
 */
export async function clickFirstPhoto(page: Page) {
  // Get coordinates for the first photo
  const coordinates = await getRegionCenter(page, 1);
  if (!coordinates) {
    throw new Error('No photos found in scene');
  }
  
  await clickPhotoAtCoordinates(page, coordinates.x, coordinates.y);
}

/**
 * Wait for the scene to be loaded and ready
 */
export async function waitForSceneLoaded(page: Page): Promise<void> {
  await getCurrentScene(page);
}

/**
 * Get current scene ID directly from the global window variable
 */
export async function getCurrentSceneId(page: Page): Promise<string | null> {
  return (await getCurrentScene(page)).id || null;
}

/**
 * Get the complete current scene data from the global window variable
 */
export async function getCurrentScene(page: Page): Promise<Scene> {
  const handle = await page.waitForFunction(() => {
    const scene = window.__PHOTOFIELD__?.currentScene;
    console.log(window.__PHOTOFIELD__);
    if (!scene || scene.loading) return null;
    return scene;
  }, { timeout: 3000 });
  return await handle.jsonValue() as Scene;
}

/**
 * Check if a photo is currently focused (URL contains region ID)
 */
export async function waitForImmersiveMode(page: Page) {
  // Wait until header.immersive is present
  await page.waitForSelector('header.immersive');
}

/**
 * Get the current focus zoom level
 */
export async function getFocusZoom(page: Page): Promise<number> {
  return await page.evaluate(() => {
    // Try to access the Vue component state
    const tileViewer = document.querySelector('.tileViewer');
    if (tileViewer && (tileViewer as any).__vueParentComponent) {
      const ctx = (tileViewer as any).__vueParentComponent.ctx;
      return ctx.focusZoom || 0;
    }
    
    // Fallback: check if we're in focused state based on URL and viewport
    const isRegionUrl = /\/regions\/\d+/.test(window.location.href);
    return isRegionUrl ? 1 : 0;
  });
}

/**
 * Perform a swipe gesture on the tile viewer
 */
export async function swipeOnViewer(
  page: Page,
  direction: 'left' | 'right' | 'up' | 'down',
  distance: number = 100
): Promise<void> {
  const viewer = page.locator('.tileViewer');
  const box = await viewer.boundingBox();
  
  if (!box) {
    throw new Error('Could not get tile viewer bounds');
  }
  
  const centerX = box.x + box.width / 2;
  const centerY = box.y + box.height / 2;
  
  let endX = centerX;
  let endY = centerY;
  
  switch (direction) {
    case 'left':
      endX = centerX - distance;
      break;
    case 'right':
      endX = centerX + distance;
      break;
    case 'up':
      endY = centerY - distance;
      break;
    case 'down':
      endY = centerY + distance;
      break;
  }
  
  await page.mouse.move(centerX, centerY);
  await page.mouse.down();
  await page.mouse.move(endX, endY, { steps: 10 });
  await page.mouse.up();
}
