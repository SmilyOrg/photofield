import { Page, expect } from '@playwright/test';
import { createBdd } from 'playwright-bdd';
import { DataTable } from '@cucumber/cucumber';
import { test, App } from './fixtures';
import fs from 'fs/promises';
import path from 'path';


const { Given, When, Then } = createBdd(test);

Given('an empty working directory', async ({ app }) => {
  await app.useTempDir();
  console.log("CWD:", app.cwd);
});


Given('the config {string}', async ({ app }, p: string) => {
  const configPath = path.resolve(__dirname, "..", "configs", p);
  await fs.copyFile(configPath, app.path("configuration.yaml"));
});

async function addFiles(dataTable: DataTable, app: App) {
  for (const row of dataTable.rows()) {
    const [src, dst] = row;
    const srcPath = path.resolve(__dirname, "..", src);
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
});

Given('a running app', async ({ app }) => {
  await app.run();
  await expect(async () => {
    expect(app.stderr).toContain("app running");
  }).toPass();
});

When('the API goes down', async ({ app }) => {
  await app.stop();
});

When('the user stops the app', async ({ app }) => {
  await app.stop();
});

When('the API comes back up', async ({ app }) => {
  await app.run();
});

Then('debug wait {int}', async ({}, ms: number) => {
  await new Promise(resolve => setTimeout(resolve, ms));
});

Then('the app logs {string}', async ({ app }, log: string) => {
  await expect(async () => {
    expect(app.stderr).toContain(log);
  }).toPass();
});

When('the user waits for {int} seconds', async ({ page }, sec: number) => { 
  await page.waitForTimeout(sec * 1000);
});

When('waits a second', async ({ page }) => { 
  await page.waitForTimeout(1000);
});

When('the user opens the home page', async ({ app }) => {
  await app.goto("/");
});

When('the user opens {string}', async ({ app }, path: string) => {
  await app.goto(path);
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

When('the user clicks {string}', async ({ page }, text: string) => {
  await page.getByText(text).first().click();
});

When('the user adds a folder {string}', async ({ app }, name: string) => {
  await app.addDir(name);
});

When('the user clicks "Retry', async ({ page }) => {
  await page.getByRole('button', { name: 'Retry' }).click();
});


Then('the file {string} exists', async ({ app }, filePath: string) => {
  await fs.stat(app.path(filePath));
});

Then('the file {string} does not exist', async ({ app }, filePath: string) => {
  try {
    await fs.stat(app.path(filePath));
    throw new Error("File exists");
  } catch (error) {
    expect(error.code).toBe('ENOENT');
  }
});

Then('the page shows photo {string}', async ({ app, page }, path: string) => {
  
});