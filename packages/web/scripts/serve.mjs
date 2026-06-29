import { createReadStream } from "node:fs";
import { stat } from "node:fs/promises";
import http from "node:http";
import path from "node:path";
import { distRoot, exportWiki, webRoot } from "./wiki-export.mjs";

const root = process.env.OPENKNOWLEDGE_WEB_ROOT === "dist" ? distRoot : webRoot;
const port = Number(process.env.PORT || 4173);
const host = process.env.HOST || "127.0.0.1";
const refreshWiki = process.env.OPENKNOWLEDGE_WEB_EXPORT_WIKI !== "0";

const types = new Map([
  [".css", "text/css; charset=utf-8"],
  [".html", "text/html; charset=utf-8"],
  [".js", "text/javascript; charset=utf-8"],
  [".json", "application/json; charset=utf-8"],
  [".png", "image/png"],
  [".txt", "text/plain; charset=utf-8"]
]);

function installRedirectLocation(url) {
  const parsed = new URL(url, `http://127.0.0.1:${port}`);
  if (parsed.pathname !== "/install" && parsed.pathname !== "/install/") {
    return "";
  }
  return "https://github.com/openknowledge-sh/openknowledge/releases/latest/download/install";
}

function isInsideRoot(rootDir, target) {
  const relative = path.relative(rootDir, target);
  return relative === "" || (!relative.startsWith("..") && !path.isAbsolute(relative));
}

function candidatePath(rootDir, pathname) {
  const target = path.normalize(path.join(rootDir, pathname === "/" ? "index.html" : pathname));
  return isInsideRoot(rootDir, target) ? target : null;
}

function filePathsForUrl(url) {
  const parsed = new URL(url, `http://127.0.0.1:${port}`);
  const pathname = decodeURIComponent(parsed.pathname);
  const candidates = [candidatePath(root, pathname)];

  if (root !== distRoot && (pathname === "/wiki" || pathname.startsWith("/wiki/"))) {
    candidates.push(candidatePath(distRoot, pathname));
  }

  return candidates.filter(Boolean);
}

function commandAliasLocation(url) {
  const parsed = new URL(url, `http://127.0.0.1:${port}`);
  const pathname = decodeURIComponent(parsed.pathname);
  const match = pathname.match(/^\/wiki\/([A-Za-z0-9_-]+)(?:\.html)?\/?$/);
  if (!match) {
    return "";
  }
  return `/wiki/features/commands/${match[1]}.html${parsed.search}`;
}

const server = http.createServer(async (request, response) => {
  const installLocation = installRedirectLocation(request.url || "/");
  if (installLocation) {
    response.writeHead(302, {
      Location: installLocation
    });
    response.end();
    return;
  }

  const filePaths = filePathsForUrl(request.url || "/");
  if (filePaths.length === 0) {
    response.writeHead(403);
    response.end("Forbidden");
    return;
  }

  for (const filePath of filePaths) {
    try {
      const info = await stat(filePath);
      const target = info.isDirectory() ? path.join(filePath, "index.html") : filePath;
      const targetInfo = info.isDirectory() ? await stat(target) : info;

      if (!targetInfo.isFile()) {
        continue;
      }

      response.writeHead(200, {
        "Content-Type": types.get(path.extname(target)) || "application/octet-stream"
      });
      createReadStream(target).pipe(response);
      return;
    } catch {
      // Try the next candidate, such as the generated dist wiki in source mode.
    }
  }

  const alias = commandAliasLocation(request.url || "/");
  if (alias) {
    response.writeHead(302, {
      Location: alias
    });
    response.end();
    return;
  }

  response.writeHead(404);
  response.end("Not found");
});

if (refreshWiki) {
  await exportWiki(path.join(distRoot, "wiki"));
}

server.listen(port, host, () => {
  console.log(`Open Knowledge web: http://${host}:${port}`);
});
