import { copyFile, mkdir, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const sourceRoot = path.join(webRoot, "viewer-dist");
const targetRoot = path.resolve(webRoot, "../cli/cmd/openknowledge/viewer_assets");
const assets = ["viewer.js", "viewer.css", "viewer-theme.js"];

await mkdir(targetRoot, { recursive: true });
for (const asset of assets) {
  const source = path.join(sourceRoot, asset);
  await readFile(source);
  await copyFile(source, path.join(targetRoot, asset));
}

console.log(`Synchronized viewer assets to ${targetRoot}`);
