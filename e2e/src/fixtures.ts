// Note: import base from playwright-bdd, not from @playwright/test!
import { test as base } from 'playwright-bdd';
import fs from 'fs/promises';
import { join } from 'path';
import { ChildProcess, spawn, SpawnOptionsWithoutStdio } from 'child_process';
import { BrowserContext, expect, Page } from '@playwright/test';
import { globalCache } from '@vitalets/global-cache';

const LISTEN_REGEX = /local\s+http:\/\/(\S+)/;

// Type definitions for photo testing
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

// Extend the Window interface to include our global photofield object
declare global {
  interface Window {
    __PHOTOFIELD__?: {
      currentScene?: Scene;
    };
  }

  interface Element {
    __vueParentComponent?: {
      ctx?: {
        focusZoom?: number;
      };
      setupState?: {
        regionZoom?: number;
      };
    };
  }
}

async function spawnp(
  command: string,
  args: string[],
  options: SpawnOptionsWithoutStdio,
): Promise<{ stdout: string; stderr: string }> {
  return new Promise((resolve, reject) => {
    const proc = spawn(command, args, options);

    let stdout = '';
    let stderr = '';

    proc.stdout?.on('data', (data) => {
      const msg = data.toString();
      console.log(msg);
      stdout += msg;
    });

    proc.stderr?.on('data', (data) => {
      const msg = data.toString();
      console.error(msg);
      stderr += msg;
    });

    proc.on('close', (code) => {
      if (code === 0) {
        resolve({ stdout, stderr });
      } else {
        reject(new Error(`Process failed with code ${code}: ${stderr}`));
      }
    });

    proc.on('error', (err) => {
      reject(new Error(`Failed to spawn process: ${err.message}`));
    });
  });
}

export class App {

  public cwd: string;
  public stdout: string;
  public stderr: string;
  public host: string = 'localhost';
  public port: number = 0;
  public listenHost: string = '';
  public disableAutostart: boolean = true;
  public dataDir: string = '.';
  proc?: ChildProcess;
  exitCode: number | null;
  uiLocal: boolean = true;
  public uiUrl: string = "http://localhost:5173";
  public collectionPath: string = "";
  public testScrollPosition: number = 0;

  constructor(
    public page: Page,
    public context: BrowserContext,
  ) {}

  async useTempDir() {
    const tmpDir = 'test-tmp/'
    // Format like 2021-09-30-12-00-00-000
    const datetime = new Date().toISOString()
      .replace(/Z/, '')
      .replace(/[:T.]/g, '-');
    const suffix = Math.random().toString(36).substring(2, 8);  
    const dirName = `test-${datetime}-${suffix}`;
    this.cwd = join(process.cwd(), tmpDir, dirName);
    await fs.mkdir(this.cwd, { recursive: true });
  }

  async addDir(dir: string) {
    console.log("Adding dir:", dir);
    await fs.mkdir(join(this.cwd, dir), { recursive: true });
  }

  path(path: string) {
    return join(this.cwd, path);
  }

  async generatePhotos(count: number, seed: number = 12345, widths: number[], heights: number[]): Promise<void> {
    // Use global cache to generate test data only once per run
    const widthKey = widths.join('_');
    const heightKey = heights.join('_');
    const cacheKey = `e2e-test-${count}-${widthKey}-${heightKey}-${seed}`;

    console.log("Using generated photos cache key:", cacheKey);
    
    const cache = await globalCache.get(cacheKey, async () => {
      console.log(`Generating ${count} test ${widths.join(',')} x ${heights.join(',')} photos with seed ${seed}...`);
      
      const exe = process.platform === 'win32' ? '.exe' : '';
      const command = join(process.cwd(), '..', 'photofield' + exe);
      
      // Use testdata as the output directory
      const outputDir = join(process.cwd(), '..', 'testdata');
      await fs.mkdir(outputDir, { recursive: true });

      const args = [
        '-gen-photos',
        '-gen-photos.count', count.toString(),
        '-gen-photos.seed', seed.toString(),
        '-gen-photos.name', cacheKey,
        '-gen-photos.output', outputDir,
      ];
      args.push('-gen-photos.widths', widths.join(','));
      args.push('-gen-photos.heights', heights.join(','));
      
      console.log("Generating photos:", command, args);
      await spawnp(command, args, {
        cwd: outputDir,
        stdio: 'pipe',
        timeout: 30000,
      });

      const dataDir = join(outputDir, cacheKey);

      console.log("Generating database");
      await spawnp(command, ['-scan', cacheKey], {
        cwd: outputDir,
        stdio: 'pipe',
        timeout: 30000,
        env: {
          PATH: process.env.PATH,
          PHOTOFIELD_DATA_DIR: dataDir,
        }
      });

      console.log("Vacuuming database");
      await spawnp(command, ['-vacuum'], {
        cwd: outputDir,
        stdio: 'pipe',
        timeout: 30000,
        env: {
          PATH: process.env.PATH,
          PHOTOFIELD_DATA_DIR: dataDir,
        }
      });
      return {
        outputDir,
        dataDir,
      };
    });

    const { dataDir: generatedDataDir, outputDir } = cache;
    
    // Set the generated paths from cache
    this.dataDir = this.cwd || process.cwd();
    this.cwd = outputDir;
    this.collectionPath = `/collections/${cacheKey}`;

    // Copy database files from the generated data dir
    const files = [
      "photofield.cache.db",
      "photofield.thumbs.db",
    ]
    await Promise.all(files.map(file => {
      return fs.copyFile(join(generatedDataDir, file), join(this.dataDir, file));
    }));
  }

  async run(args: string[] = []) {
    const exe = process.platform === 'win32' ? '.exe' : '';
    const command = join(process.cwd(), '../photofield' + exe);

    const address = `${this.host}:${this.port}`;

    const env = {
      PATH: process.env.PATH,
      PHOTOFIELD_ADDRESS: this.listenHost || address,
      PHOTOFIELD_API_PREFIX: '/',
      PHOTOFIELD_CORS_ALLOWED_ORIGINS: 'http://localhost:5173',
      PHOTOFIELD_DATA_DIR: this.dataDir,
    };

    console.log("Running:", command, args, env);

    this.proc = spawn(command, args, {
      cwd: this.cwd,
      env,
      stdio: 'pipe',
      timeout: 60000,
    });
    this.proc.stdout!.on('data', (data) => {
      console.log(data.toString());
      this.stdout += data.toString();
    });
    this.proc.stderr!.on('data', (data) => {
      const msg = data.toString();
      console.error(msg);
      if (msg.includes('api only')) {
        console.log("API only mode, using local UI")
        if (!this.uiUrl) {
          this.uiLocal = true;
          this.uiUrl = `http://localhost:5173`;
        }
      }
      const match = msg.match(LISTEN_REGEX);
      if (match) {
        this.listenHost = match[1];
        if (!this.uiUrl) {
          this.uiUrl = "http://" + this.listenHost;
        }
      }
      if (!this.stderr) {
        this.stderr = "";
      }
      this.stderr += msg;
    });
    this.proc.on('close', (code) => {
      this.exitCode = code;
    });
  }

  async goto(path: string) {
    if (this.uiLocal) {
      await this.context.addCookies([
        {
          name: 'photofield-api-host',
          value: "http://" + (this.listenHost || "localhost:99999"),
          url: this.uiUrl || "http://localhost:5173",
        }
      ]);
      console.log(await this.context.cookies())
    }
    await this.page.goto(`${this.uiUrl}${path}`);
  }

  async stop() {
    this.proc?.kill('SIGTERM');
    this.proc = undefined;
  }

  async cleanup() {
    this.stop();
  }

  /**
   * Get the API host from the page context
   */
  async getApiHost(): Promise<string> {
    return await this.page.evaluate(() => {
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
   * Get photo regions from the API
   */
  async getPhotoRegions(
    sceneId?: string,
    bounds?: { x: number; y: number; w: number; h: number },
    limit?: number
  ): Promise<PhotoRegion[]> {
    // Get scene ID if not provided
    const actualSceneId = sceneId || await this.getCurrentSceneId();
    if (!actualSceneId) {
      throw new Error('No scene ID available');
    }
    
    const apiHost = await this.getApiHost();
    
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
    
    const response = await this.page.evaluate(async (url) => {
      const res = await fetch(url);
      return res.json();
    }, url);
    
    return response.items || [];
  }

  /**
   * Get the closest photo to specific coordinates
   */
  async getClosestPhoto(
    x: number,
    y: number,
    sceneId?: string
  ): Promise<PhotoRegion | null> {
    // Get scene ID if not provided
    const actualSceneId = sceneId || await this.getCurrentSceneId();
    if (!actualSceneId) {
      throw new Error('No scene ID available');
    }
    
    const apiHost = await this.getApiHost();
    const url = `${apiHost}/scenes/${actualSceneId}/regions?x=${x}&y=${y}&closest=true&limit=1`;
    
    const response = await this.page.evaluate(async (url) => {
      const res = await fetch(url);
      return res.json();
    }, url);
    
    return response.items?.[0] || null;
  }

  /**
   * Convert scene coordinates to pixel coordinates for clicking
   */
  async sceneToPixelCoordinates(
    sceneX: number,
    sceneY: number
  ): Promise<PixelCoordinates | null> {
    return await this.page.evaluate(({ sceneX, sceneY }) => {
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
  async clickPhotoAtCoordinates(
    sceneX: number,
    sceneY: number
  ): Promise<void> {
    const pixelCoords = await this.sceneToPixelCoordinates(sceneX, sceneY);
    if (!pixelCoords) {
      throw new Error('Could not convert scene coordinates to pixel coordinates');
    }
    
    await expect(async () => {
      await this.page.mouse.click(pixelCoords.x, pixelCoords.y);
      await expect(this.page.locator('header.immersive')).toBeVisible({ timeout: 500 });
    }).toPass();
  }

  /**
   * Get coordinates for a photo region (center point) using direct API call
   */
  async getRegionCenter(
    regionId: number,
    sceneId?: string
  ): Promise<SceneCoordinates | null> {
    const region = await this.getRegion(regionId, sceneId);
    return {
      x: region.bounds.x + region.bounds.w / 2,
      y: region.bounds.y + region.bounds.h / 2
    };
  }

  /**
   * Wait for a specific photo region to be available in the scene and return its data.
   */
  async getRegion(regionId: number, sceneId?: string): Promise<PhotoRegion> {
    const apiHost = await this.getApiHost();
    
    let region: PhotoRegion | undefined;
    await expect(async () => {
      const actualSceneId = sceneId || await this.getCurrentSceneId();
      if (!actualSceneId) {
        throw new Error('No scene ID available');
      }

      const url = `${apiHost}/scenes/${actualSceneId}/regions/${regionId}`;
      const response = await this.page.request.get(url);
      expect(response.ok).toBeTruthy();
      region = await response.json();
      expect(region).toBeDefined();
      expect(region?.id).toBeGreaterThan(0);
      expect(region?.bounds).toBeDefined();
      expect(region?.bounds?.w).toBeGreaterThan(0);
      expect(region?.bounds?.h).toBeGreaterThan(0);
    }).toPass();
    if (!region) {
      throw new Error(`Region ${regionId} not found`);
    }
    return region;
  }

  /**
   * Click on the first photo in the scene
   */
  async clickFirstPhoto() {
    // Get coordinates for the first photo
    const coordinates = await this.getRegionCenter(1);
    if (!coordinates) {
      throw new Error('No photos found in scene');
    }
    
    await this.clickPhotoAtCoordinates(coordinates.x, coordinates.y);
  }

  /**
   * Wait for the scene to be loaded and ready
   */
  async waitForSceneLoaded(): Promise<void> {
    await this.getCurrentScene();
  }

  /**
   * Get current scene ID directly from the global window variable
   */
  async getCurrentSceneId(): Promise<string | null> {
    return (await this.getCurrentScene()).id || null;
  }

  /**
   * Get the complete current scene data from the global window variable
   */
  async getCurrentScene(): Promise<Scene> {
    const handle = await this.page.waitForFunction(() => {
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
  async waitForImmersiveMode() {
    // Wait until header.immersive is present
    await this.page.waitForSelector('header.immersive');
  }
  
  /**
   * Get the current focus zoom level
   */
  async getFocusZoom(): Promise<number> {
    return await this.page.evaluate(() => {
      // Try to access the Vue component state
      const tileViewer = document.querySelector('.tileViewer');
      if (!tileViewer) return 0;
      const focusZoom = tileViewer.__vueParentComponent?.ctx?.focusZoom;
      console.log("Focus zoom:", document.querySelector('.tileViewer')?.__vueParentComponent?.ctx?.focusZoom);
      return focusZoom || 0;
    });
  }

  /**
   * Perform a swipe gesture on the tile viewer
   */
  async swipeOnViewer(
    direction: 'left' | 'right' | 'up' | 'down',
    distance: number = 100
  ): Promise<void> {
    const viewer = this.page.locator('.tileViewer');
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
    
    await this.page.mouse.move(centerX, centerY);
    await this.page.mouse.down();
    await this.page.mouse.move(endX, endY, { steps: 10 });
    await this.page.mouse.up();
  }
}

// export custom test function
export const test = base.extend<{ app: App }>({
  app: async ({ page, context }, use) => {
    const app = new App(page, context);
    await use(app);
    await app.cleanup();
  }
});