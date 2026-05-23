import { execFile } from "child_process";
import { promisify } from "util";

const execFileAsync = promisify(execFile);

async function globalTeardown() {
  if (process.platform === "win32") {
    await killExiftool();
  }
}

// Workaround of photofield not cleaning up exiftool.exe properly
async function killExiftool() {
  await execFileAsync("taskkill", ["/F", "/IM", "exiftool.exe"]);
}

export default globalTeardown;
