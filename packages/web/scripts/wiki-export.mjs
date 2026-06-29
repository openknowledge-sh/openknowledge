import { spawn } from "node:child_process";
import { rm } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptsRoot = path.dirname(fileURLToPath(import.meta.url));

export const webRoot = path.resolve(scriptsRoot, "..");
export const repoRoot = path.resolve(webRoot, "../..");
export const distRoot = path.join(webRoot, "dist");
export const wikiRoot = path.join(repoRoot, "Wiki");

export async function run(command, args) {
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
  const openknowledgeBin = process.env.OPENKNOWLEDGE_BIN?.trim();
  if (openknowledgeBin) {
    await run(openknowledgeBin, args);
    return;
  }

  await run("go", ["run", "./packages/cli/cmd/openknowledge", ...args]);
}

export async function exportWiki(out = path.join(distRoot, "wiki"), options = {}) {
  if (options.clean !== false) {
    await rm(out, { recursive: true, force: true });
  }

  await runOpenKnowledge(["to", "html", "--out", out, wikiRoot]);
}
