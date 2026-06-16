import { cp, mkdir, rm } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(root, "..");
const projectRoot = path.resolve(webRoot, "../..");
const dist = path.join(webRoot, "dist");

await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });
await cp(path.join(webRoot, "index.html"), path.join(dist, "index.html"));
await cp(path.join(webRoot, "main.js"), path.join(dist, "main.js"));
await cp(path.join(webRoot, "styles.css"), path.join(dist, "styles.css"));
await cp(path.join(webRoot, "CNAME"), path.join(dist, "CNAME"));
await cp(path.join(webRoot, ".nojekyll"), path.join(dist, ".nojekyll"));
await cp(path.join(projectRoot, "install"), path.join(dist, "install"));

console.log(`Built ${dist}`);
