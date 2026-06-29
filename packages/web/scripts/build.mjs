import { cp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(root, "..");
const dist = path.join(webRoot, "dist");
const headMarker = "<!-- OPENKNOWLEDGE_HEAD -->";

await rm(dist, { recursive: true, force: true });
await mkdir(dist, { recursive: true });
await writeFile(
  path.join(dist, "index.html"),
  await injectHeadHTML(await readFile(path.join(webRoot, "index.html"), "utf8")),
);
await cp(path.join(webRoot, "main.js"), path.join(dist, "main.js"));
await cp(path.join(webRoot, "favicon.png"), path.join(dist, "favicon.png"));
await cp(path.join(webRoot, "apple-touch-icon.png"), path.join(dist, "apple-touch-icon.png"));
await cp(path.join(webRoot, "og.png"), path.join(dist, "og.png"));
await cp(path.join(webRoot, "styles.css"), path.join(dist, "styles.css"));

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
