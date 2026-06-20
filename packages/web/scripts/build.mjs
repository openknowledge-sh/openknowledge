import { spawn } from "node:child_process";
import { access, cp, mkdir, rm } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(root, "..");
const repoRoot = path.resolve(webRoot, "../..");
const dist = path.join(webRoot, "dist");
const wikiRoot = path.join(repoRoot, "Wiki");
const openknowledgeBin = process.env.OPENKNOWLEDGE_BIN || path.join(repoRoot, "bin", process.platform === "win32" ? "openknowledge.exe" : "openknowledge");

async function exists(file) {
  try {
    await access(file);
    return true;
  } catch {
    return false;
  }
}

async function run(command, args) {
  await new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd: repoRoot,
      stdio: "inherit"
    });

    child.on("error", reject);
    child.on("exit", (code, signal) => {
      if (code === 0) {
        resolve();
        return;
      }

      reject(new Error(`${command} ${args.join(" ")} failed with ${signal || `exit code ${code}`}`));
    });
  });
}

async function runOpenKnowledge(args) {
  if (await exists(openknowledgeBin)) {
    await run(openknowledgeBin, args);
    return;
  }

  await run("go", ["run", "./packages/cli/cmd/openknowledge", ...args]);
}

await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });

for (const asset of ["index.html", "main.js", "favicon.png", "apple-touch-icon.png", "og.png", "styles.css"]) {
  await cp(path.join(webRoot, asset), path.join(dist, asset));
}

await runOpenKnowledge(["to", "html", "--out", path.join(dist, "wiki"), wikiRoot]);

console.log(`Built ${dist}`);
