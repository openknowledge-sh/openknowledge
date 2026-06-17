import { cp, mkdir, rm } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(root, "..");
const dist = path.join(webRoot, "dist");

await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });
await cp(path.join(webRoot, "index.html"), path.join(dist, "index.html"));
await cp(path.join(webRoot, "main.js"), path.join(dist, "main.js"));
await cp(path.join(webRoot, "favicon.png"), path.join(dist, "favicon.png"));
await cp(path.join(webRoot, "apple-touch-icon.png"), path.join(dist, "apple-touch-icon.png"));
await cp(path.join(webRoot, "og.png"), path.join(dist, "og.png"));
await cp(path.join(webRoot, "styles.css"), path.join(dist, "styles.css"));

console.log(`Built ${dist}`);
