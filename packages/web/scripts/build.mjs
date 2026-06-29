import { cp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { distRoot as dist, exportWiki, webRoot } from "./wiki-export.mjs";

const headMarker = "<!-- OPENKNOWLEDGE_HEAD -->";

await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });

for (const asset of ["index.html", "main.js", "favicon.png", "apple-touch-icon.png", "og.png", "styles.css", "robots.txt"]) {
  if (asset === "index.html") {
    await writeFile(path.join(dist, asset), await injectHeadHTML(await readFile(path.join(webRoot, asset), "utf8")));
  } else {
    await cp(path.join(webRoot, asset), path.join(dist, asset));
  }
}

await exportWiki(path.join(dist, "wiki"), { clean: false });

console.log(`Built ${dist}`);

async function injectHeadHTML(html) {
  const headHTML = await loadHeadHTML();
  return html.replace(headMarker, headHTML);
}

async function loadHeadHTML() {
  const snippets = [];
  const headFile = (process.env.OPENKNOWLEDGE_HEAD_FILE || "").trim();
  const inlineHead = (process.env.OPENKNOWLEDGE_HEAD_HTML || "").trim();
  const scriptSrcs = splitScriptSrcs(process.env.OPENKNOWLEDGE_SCRIPT_SRC || "");

  if (headFile) {
    snippets.push(await readFile(path.resolve(process.cwd(), headFile), "utf8"));
  }
  if (inlineHead) {
    snippets.push(inlineHead);
  }
  for (const src of scriptSrcs) {
    snippets.push(scriptTag(src));
  }

  return snippets.join("\n    ");
}

function splitScriptSrcs(value) {
  return value
    .split(/[,\n\r]/)
    .map((part) => part.trim())
    .filter(Boolean);
}

function scriptTag(src) {
  if (!validScriptSrc(src)) {
    throw new Error(`Unsupported script src: ${src}`);
  }
  return `<script src="${escapeHTML(src)}"></script>`;
}

function validScriptSrc(src) {
  if (!src) {
    return false;
  }
  const scheme = src.match(/^[A-Za-z][A-Za-z0-9+.-]*:/);
  return !scheme || scheme[0] === "http:" || scheme[0] === "https:";
}

function escapeHTML(value) {
  return value.replaceAll("&", "&amp;").replaceAll('"', "&quot;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");
}
