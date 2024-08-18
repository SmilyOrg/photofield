// Note: import base from playwright-bdd, not from @playwright/test!
import { test as base } from 'playwright-bdd';
import fs from 'fs/promises';
import { join } from 'path';
import { ChildProcess, spawn } from 'child_process';
import { BrowserContext, Page } from '@playwright/test';

const LISTEN_REGEX = /local\s+http:\/\/(\S+)/;

export class App {

  public cwd: string;
  public stdout: string;
  public stderr: string;
  public host: string = 'localhost';
  public port: number = 0;
  public listenHost: string = '';
  public disableAutostart: boolean = true;
  proc?: ChildProcess;
  exitCode: number | null;
  uiLocal: boolean = true;
  uiUrl: string = "http://localhost:3000";

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
    await fs.mkdir(this.cwd);
  }

  async addDir(dir: string) {
    console.log("Adding dir:", dir);
    await fs.mkdir(join(this.cwd, dir));
  }

  path(path: string) {
    return join(this.cwd, path);
  }

  async run() {
    const exe = process.platform === 'win32' ? '.exe' : '';
    const command = join(process.cwd(), '../photofield' + exe);

    const address = `${this.host}:${this.port}`;

    const env = {
      PHOTOFIELD_ADDRESS: this.listenHost || address,
      PHOTOFIELD_API_PREFIX: '/',
      PHOTOFIELD_CORS_ALLOWED_ORIGINS: 'http://localhost:3000',
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
      const msg = data.toString();
      console.error(msg);
      if (msg.includes('api only')) {
        console.log("API only mode, using local UI")
        if (!this.uiUrl) {
          this.uiLocal = true;
          this.uiUrl = `http://localhost:3000`;
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
          url: this.uiUrl || "http://localhost:3000",
        }
      ]);
      console.log(await this.context.cookies())
    }
    await this.page.goto(`${this.uiUrl}${path}`);
  }

  async stop() {
    this.proc?.kill('SIGTERM');
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
  app: async ({ page, context }, use) => {
    const app = new App(page, context);
    await use(app);
    await app.cleanup();
  }
});