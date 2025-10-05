import { Page, expect } from '@playwright/test';
import { createBdd } from 'playwright-bdd';
import { test, App } from './fixtures';
import { promises as fs } from 'fs';
import * as path from 'path';

interface DataTable {
  rows(): string[][];
}

const { Given, When, Then } = createBdd(test);

Given('an empty working directory', async ({ app }) => {
  await app.useTempDir();
  console.log("CWD:", app.cwd);
});

Given('{int} generated {int} x {int} test photos', async ({ app }, count: number, width: number, height: number) => {
  if (!app.cwd) {
    await app.useTempDir();
    console.log("CWD:", app.cwd);
  }
  await app.generatePhotos(count, 12345, [width], [height]);
});

Given('the config {string}', async ({ app }, p: string) => {
  const configPath = path.resolve(__dirname, "..", "configs", p);
  await fs.copyFile(configPath, app.path("configuration.yaml"));
});

When('the user adds the config {string}', async ({ app }, p: string) => {
  const configPath = path.resolve(__dirname, "..", "configs", p);
  await fs.copyFile(configPath, app.path("configuration.yaml"));
});

async function addFiles(dataTable: DataTable, app: App) {
  if (!app.cwd) {
    await app.useTempDir();
    console.log("CWD:", app.cwd);
  }
  for (const row of dataTable.rows()) {
    const [src, dst] = row;
    const srcPath = path.resolve(src);
    const dstPath = app.path(dst);
    await fs.mkdir(path.dirname(dstPath), { recursive: true });
    await fs.copyFile(srcPath, dstPath);
  }
}

Given('the following files:', async ({ app }, dataTable: DataTable) => {
  await addFiles(dataTable, app);
});

When('the user adds the following files:', async ({ app }, dataTable: DataTable) => {
  await addFiles(dataTable, app);
});

When('the user runs the app', async ({ app }) => {
  await app.run();
  await expect(async () => {
    expect(app.stderr).toContain("app running");
  }).toPass();
});

Given('a running app', async ({ app }) => {
  await app.run();
  await expect(async () => {
    expect(app.stderr).toContain("app running");
  }).toPass();
});

Given('no running app', async ({ app }) => {
  app.disableAutostart = true;
  await app.stop();
});

When('the API goes down', async ({ app }) => {
  await app.stop();
});

When('the user stops the app', async ({ app }) => {
  await app.stop();
});

When('the API comes back up', async ({ app }) => {
  await app.run();
  await expect(async () => {
    expect(app.stderr).toContain("app running");
  }).toPass();
});

Then('debug wait {int}', async ({}, ms: number) => {
  await new Promise(resolve => setTimeout(resolve, ms));
});

Then('the app logs {string}', async ({ app }, log: string) => {
  await expect(async () => {
    expect(app.stderr).toContain(log);
  }).toPass();
});

When('the user waits( for) {int} second(s)', async ({ page }, sec: number) => { 
  await page.waitForTimeout(sec * 1000);
});

When('(the user )waits a second', async ({ page }) => { 
  await page.waitForTimeout(1000);
});

When('(the user )opens the home page', async ({ app }) => {
  if (!app.proc && !app.disableAutostart) {
    await app.run();
    await expect(async () => {
      expect(app.stderr).toContain("app running");
    }
    ).toPass();
  }
  await app.goto("/");
});

When('the user opens {string}', async ({ app }, path: string) => {
  await app.goto(path);
});

When('the user opens the collection', async ({ app }) => {
  await app.goto(app.collectionPath);
});

Then('the page shows a progress bar', async ({ page }) => {
  await expect(page.locator("#content").getByRole('progressbar')).toBeVisible();
});

Then('the page shows {string}', async ({ page }, text) => {
  await expect(page.getByText(text).first()).toBeVisible();
});

Then('the page does not show {string}', async ({ page }, text: string) => {
  await expect(page.getByText(text)).not.toBeVisible();
});

When('the user switches away and back to the page', async ({ page }) => {
  await page.evaluate(() => {
    document.dispatchEvent(new Event('visibilitychange'))
  })
});

When('(the user )clicks {string}', async ({ page }, text: string) => {
  await page.getByText(text).first().click();
});

When('the user adds a folder {string}', async ({ app }, name: string) => {
  await app.addDir(name);
});

Then('the file {string} exists', async ({ app }, filePath: string) => {
  await fs.stat(app.path(filePath));
});

Then('the file {string} does not exist', async ({ app }, filePath: string) => {
  try {
    await fs.stat(app.path(filePath));
    throw new Error("File exists");
  } catch (error) {
    console.log("Error:", error);
    expect(error.code === 'ENOENT' || error.message.includes('no such file or directory')).toBe(true);
  }
});

// Photo interaction and navigation steps
When('(the user )clicks on the first photo', async ({ app }) => {
  await app.clickFirstPhoto();
});

Then('the page loads', async ({ app }) => {
  await app.waitForSceneLoaded();
});

When('the user clicks on a photo at scene coordinates {int}, {int}', async ({ app }, x: number, y: number) => {
  await app.clickPhotoAtCoordinates(x, y);
});

Then('the photo is focused and zoomed in', async ({ app }) => {
  // Check that we're in focus mode (URL should contain photo ID)
  await app.waitForImmersiveMode();

  // Wait until the focus zoom level is greater than 0.9
  await expect(async () => {
    expect(await app.getFocusZoom()).toBeGreaterThan(0.9);
  }).toPass();
});

Then('the path is {string}', async ({ app, page }, expectedUrl: string) => {
  await expect(page).toHaveURL(app.uiUrl + expectedUrl);
});

Then('the collection subpath is {string}', async ({ app, page }, expectedSubpath: string) => {
  await expect(page).toHaveURL(app.uiUrl + app.collectionPath + expectedSubpath);
});

Then('the url contains {string}', async ({ app, page }, substring: string) => {
  await expect(async () => {
    expect(page.url()).toContain(substring);
  }).toPass();
});

Then('the url does not contain {string}', async ({ app, page }, substring: string) => {
  await expect(async () => {
    expect(page.url()).not.toContain(substring);
  }).toPass();
});

When('the user clicks on the info icon', async ({ page }) => {
  // Click on the info icon in the controls toolbar
  await page.locator('.controls .toolbar .icon:has-text("info_outline")').click();
});

When('(the user )presses the {string} key', async ({ page }, key: string) => {
  await page.keyboard.press(key);
});

When('(the user )swipes left on the photo viewer', async ({ app }) => {
  await app.swipeOnViewer('left', 400);
});

Then('no photo is focused', async ({ app }) => {
  // Verify focus zoom is reset
  const focusZoom = await app.getFocusZoom();
  expect(focusZoom).toBeLessThan(0.1);
});

When('(the user )zooms in using mouse wheel', async ({ page }) => {
  const viewer = page.locator('.tileViewer');
  await viewer.hover();
  const box = await viewer.boundingBox();
  if (!box) throw new Error("Could not get bounding box for .tileViewer");
  // Move to center of page
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.wheel(0, -200); // Scroll up to zoom in
});

Then('the photo is displayed at higher magnification', async ({ app }) => {
  // Wait for zoom change to take effect
  await app.page.waitForTimeout(500);
  
  // Check that we're still focused (zoom should be > 1)
  const focusZoom = await app.getFocusZoom();
  expect(focusZoom).toBeGreaterThan(1.0);
});

When('the user drags the photo', async ({ page }) => {
  const viewer = page.locator('.tileViewer');
  const box = await viewer.boundingBox();
  
  if (box) {
    await page.mouse.move(box.x + box.width * 0.5, box.y + box.height * 0.5);
    await page.mouse.down();
    await page.mouse.move(box.x + box.width * 0.3, box.y + box.height * 0.3, { steps: 5 });
    await page.mouse.up();
  }
});

Then('the photo view pans accordingly', async ({ app }) => {
  // Wait for pan animation to complete
  await app.page.waitForTimeout(500);
  
  // The pan is successful if we didn't lose focus
  await app.waitForImmersiveMode();
});

When('the user right-clicks on a photo', async ({ app }) => {
  // First click on a photo, then right-click
  const coords = await app.getRegionCenter(1);
  if (!coords) throw new Error('No coordinates found');

  await app.page.mouse.click(coords.x, coords.y, { button: 'right' });
});

Then('a context menu appears', async ({ page }) => {
  await expect(page.locator('.region')).toBeVisible();
});

Then('the menu contains {string}', async ({ page }, text: string) => {
  await expect(page.getByText(text)).toBeVisible();
});

When('the user holds Ctrl and drags a selection box', async ({ page }) => {
  const viewer = page.locator('.tileViewer');
  const box = await viewer.boundingBox();
  
  if (box) {
    await page.keyboard.down('Control');
    await page.mouse.move(box.x + 100, box.y + 100);
    await page.mouse.down();
    await page.mouse.move(box.x + 200, box.y + 200, { steps: 5 });
    await page.mouse.up();
    await page.keyboard.up('Control');
  }
});

When('the user switches to {string} layout', async ({ app }, layout: string) => {
  // Navigate using URL parameter (most reliable method)
  const currentUrl = new URL(app.page.url());
  currentUrl.searchParams.set('layout', layout);
  await app.page.goto(currentUrl.toString());
  
  await app.waitForSceneLoaded();
});

When('the user searches for {string}', async ({ app, page }, searchTerm: string) => {
  if (!await page.getByRole('button', { name: 'Close Search' }).isVisible()) {
    await page.getByRole('button', { name: 'Open Search' }).click();
  }
  const input = page.getByRole('textbox', { name: 'Search' });
  await input.fill(searchTerm);
  await input.press('Enter');
  
  await app.waitForSceneLoaded();
});

When('the user performs a cross-drag gesture {string}', async ({ app }, direction: string) => {
  switch (direction) {
    case 'up':
      await app.swipeOnViewer('up', 100);
      break;
    case 'down':
      await app.swipeOnViewer('down', 100);
      break;
    case 'left':
      await app.swipeOnViewer('left', 150);
      break;
    case 'right':
      await app.swipeOnViewer('right', 150);
      break;
  }
});

Then('the photo collection view is shown', async ({ page }) => {
  await expect(page).toHaveURL(/\/collections\/[^\/]+$/);
});

Then('the previous photo is shown', async ({ page }) => {
  await expect(page).toHaveURL(/\/regions\/\d+/);
  // Could add additional verification that ID decreased
});

When('the page finishes loading', async ({ app }) => {
  await app.waitForSceneLoaded();
});

When('the user scrolls down {int}px', async ({ page, app }, pixels: number) => {
  // Store initial scroll position for later comparison
  const initialScrollY = await page.evaluate(() => window.scrollY);

  await expect(async () => {
    const height = await page.evaluate(() => document.body.scrollHeight);
    expect(height).toBeGreaterThanOrEqual(initialScrollY + pixels);
  }, "Page is tall enough to scroll").toPass();

  await page.waitForFunction(async ([initialScrollY, pixels]) => {
    window.scrollTo(0, initialScrollY + pixels);
    return window.scrollY === initialScrollY + pixels;
  }, [initialScrollY, pixels]);

  // Store the scroll position in the app fixture for later verification
  app.testScrollPosition = initialScrollY + pixels;
});

When('the user reloads the page', async ({ page }) => {
  await page.reload();
  await page.waitForLoadState('networkidle');
});

Then('the scroll position is roughly the same', async ({ page, app }) => {
  // Wait for page to fully load and scroll position to be restored
  await page.waitForTimeout(1000);

  const currentScrollY = await page.evaluate(() => window.scrollY);
  const expectedScrollY = app.testScrollPosition;

  // Allow for some tolerance in scroll position (within 100px)
  const tolerance = 100;
  expect(Math.abs(currentScrollY - expectedScrollY)).toBeLessThan(tolerance);
});

Then('the scroll position is {int}px', async ({ page }, expectedScrollY: number) => {
  expect(await page.evaluate(() => window.scrollY)).toBe(expectedScrollY);
});
