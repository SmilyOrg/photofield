import { promisify } from "util";
const exec = promisify(require("child_process").exec);

async function globalTeardown() {
  if (process.platform === "win32") {
    await killExiftool();
  }
}

// Workaround of photofield not cleaning up exiftool.exe properly
async function killExiftool() {
  await exec("taskkill /F /IM exiftool.exe");
}

export default globalTeardown;
