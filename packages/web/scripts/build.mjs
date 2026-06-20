import { cp, mkdir, rm } from "node:fs/promises";
import path from "node:path";
import { distRoot as dist, exportWiki, webRoot } from "./wiki-export.mjs";

await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });

for (const asset of ["index.html", "main.js", "favicon.png", "apple-touch-icon.png", "og.png", "styles.css"]) {
  await cp(path.join(webRoot, asset), path.join(dist, asset));
}

await exportWiki(path.join(dist, "wiki"), { clean: false });

console.log(`Built ${dist}`);
