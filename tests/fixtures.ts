// Note: import base from playwright-bdd, not from @playwright/test!
import { test as base } from 'playwright-bdd';
import fs from 'fs/promises';
import { join } from 'path';
import { ChildProcess, spawn } from 'child_process';
import { Page } from '@playwright/test';

const LISTEN_REGEX = /local\s+(http:\/\/\S+)/;

class App {

  public cwd: string;
  public stdout: string;
  public stderr: string;
  public host: string = 'localhost';
  public port: number = 0;
  public listenUrl: string = '';
  proc?: ChildProcess;
  exitCode: number | null;
  uiUrl: string;

  constructor(public page: Page) {
  }

  async useTempDir() {
    const tmpDir = 'test-tmp/'
    // Format like 2021-09-30-12-00-00-000
    const datetime = new Date().toISOString()
      .replace(/Z/, '')
      .replace(/[:T.]/g, '-');
    const suffix = Math.random().toString(36).substring(2, 8);  
    const dirName = `test-${datetime}-${suffix}`;
    this.cwd = join(process.cwd(), tmpDir, dirName);
    await fs.mkdir(this.cwd);
  }


  async run() {
    const exe = process.platform === 'win32' ? '.exe' : '';
    const command = join(process.cwd(), './photofield' + exe);

    const address = `${this.host}:${this.port}`;
    this.uiUrl = `http://${address}`;

    const env = {
      PHOTOFIELD_ADDRESS: address,
    };

    console.log("Running:", command, env);

    this.proc = spawn(command, [], {
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
      console.error(data.toString());
      const match = data.toString().match(LISTEN_REGEX);
      if (match) {
        this.listenUrl = match[1];
      }
      this.stderr += data.toString();
    });
    this.proc.on('close', (code) => {
      this.exitCode = code;
    });
  }

  async goto(path: string) {
    await this.page.goto(`${this.listenUrl}${path}`);
  }

  async stop() {
    this.proc?.kill();
    this.proc = undefined;
    // Remove the temporary directory
    // await fs.rmdir(this.cwd, { recursive: true });
  }

  async cleanup() {
    this.stop();
    // Remove the temporary directory
    // if (this.cwd) {
    //   await fs.rmdir(this.cwd, { recursive: true });
    // }
  }

}

// export custom test function
export const test = base.extend<{ app: App }>({
  app: async ({ page }, use) => {
    const app = new App(page);
    await use(app);
    await app.cleanup();
  }
});