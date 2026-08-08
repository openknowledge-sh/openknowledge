import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { publishCLISchemas } from "./schema-distribution.mjs";
import { distRoot as dist, exportWiki } from "./wiki-export.mjs";

const headMarker = "<!-- OPENKNOWLEDGE_HEAD -->";

const sitePages = [
  path.join(dist, "index.html"),
  path.join(dist, "getting-started", "index.html"),
  path.join(dist, "use-cases", "project-documentation", "index.html"),
];
for (const pagePath of sitePages) {
  await writeFile(pagePath, await injectHeadHTML(await readFile(pagePath, "utf8")));
}

await exportWiki(path.join(dist, "wiki"), { clean: false });
await publishCLISchemas(path.join(dist, "schemas"));

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
