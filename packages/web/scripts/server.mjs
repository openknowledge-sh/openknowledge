import { createReadStream } from "node:fs";
import { realpath, stat } from "node:fs/promises";
import http from "node:http";
import path from "node:path";

const contentTypes = new Map([
  [".css", "text/css; charset=utf-8"],
  [".gif", "image/gif"],
  [".html", "text/html; charset=utf-8"],
  [".ico", "image/x-icon"],
  [".jpeg", "image/jpeg"],
  [".jpg", "image/jpeg"],
  [".js", "text/javascript; charset=utf-8"],
  [".json", "application/json; charset=utf-8"],
  [".pdf", "application/pdf"],
  [".png", "image/png"],
  [".svg", "image/svg+xml"],
  [".txt", "text/plain; charset=utf-8"],
  [".wasm", "application/wasm"],
  [".webp", "image/webp"],
  [".woff2", "font/woff2"],
  [".xml", "application/xml; charset=utf-8"],
]);

const securityHeaders = Object.freeze({
  "Content-Security-Policy": "default-src 'self'; base-uri 'self'; connect-src 'self' https:; font-src 'self' data: https:; form-action 'self'; frame-ancestors 'none'; img-src 'self' data: https:; object-src 'none'; script-src 'self' 'unsafe-inline' https:; style-src 'self' 'unsafe-inline' https:; upgrade-insecure-requests",
  "Cross-Origin-Opener-Policy": "same-origin",
  "Permissions-Policy": "accelerometer=(), camera=(), geolocation=(), gyroscope=(), microphone=(), payment=(), usb=()",
  "Referrer-Policy": "strict-origin-when-cross-origin",
  "Strict-Transport-Security": "max-age=31536000",
  "X-Content-Type-Options": "nosniff",
  "X-Frame-Options": "DENY",
});

export function createWebServer(options) {
  const handler = createWebHandler(options);
  const server = http.createServer({ maxHeaderSize: 16 * 1024 }, handler);
  server.headersTimeout = 10_000;
  server.requestTimeout = 15_000;
  server.keepAliveTimeout = 5_000;
  server.maxRequestsPerSocket = 100;
  return server;
}

export function createWebHandler({
  root,
  fallbackRoot = "",
  installLocation = "https://github.com/openknowledge-sh/openknowledge/releases/latest/download/install",
}) {
  if (!root) {
    throw new Error("web root is required");
  }

  return async (request, response) => {
    try {
      await handleRequest(request, response, { root, fallbackRoot, installLocation });
    } catch (error) {
      if (response.headersSent) {
        response.destroy(error);
        return;
      }
      writeText(response, request.method, 500, "Internal server error");
    }
  };
}

async function handleRequest(request, response, options) {
  const method = request.method || "GET";
  if (method !== "GET" && method !== "HEAD") {
    request.resume();
    writeText(response, method, 405, "Method not allowed", { Allow: "GET, HEAD" });
    return;
  }

  const parsed = parseRequestURL(request.url || "/");
  if (!parsed) {
    writeText(response, method, 400, "Bad request");
    return;
  }

  if (parsed.pathname === "/install" || parsed.pathname === "/install/") {
    writeRedirect(response, method, options.installLocation);
    return;
  }

  const candidates = fileCandidates(parsed.pathname, options.root, options.fallbackRoot);
  if (candidates.length === 0) {
    writeText(response, method, 403, "Forbidden");
    return;
  }

  for (const candidate of candidates) {
    const file = await safeFile(candidate.root, candidate.path);
    if (!file) {
      continue;
    }
    await writeFile(response, method, file);
    return;
  }

  const alias = commandAliasLocation(parsed);
  if (alias) {
    writeRedirect(response, method, alias);
    return;
  }

  writeText(response, method, 404, "Not found");
}

function parseRequestURL(value) {
  try {
    const parsed = new URL(value, "http://127.0.0.1");
    return { pathname: decodeURIComponent(parsed.pathname), search: parsed.search };
  } catch {
    return null;
  }
}

function fileCandidates(pathname, root, fallbackRoot) {
  const candidates = [];
  const primary = candidatePath(root, pathname);
  if (primary) {
    candidates.push({ root, path: primary });
  }
  if (fallbackRoot && (pathname === "/wiki" || pathname.startsWith("/wiki/"))) {
    const fallback = candidatePath(fallbackRoot, pathname);
    if (fallback) {
      candidates.push({ root: fallbackRoot, path: fallback });
    }
  }
  return candidates;
}

function candidatePath(root, pathname) {
  const target = path.normalize(path.join(root, pathname === "/" ? "index.html" : pathname));
  return isInsideRoot(root, target) ? target : null;
}

async function safeFile(root, candidate) {
  try {
    const rootPath = await realpath(root);
    const info = await stat(candidate);
    const target = info.isDirectory() ? path.join(candidate, "index.html") : candidate;
    const resolved = await realpath(target);
    if (!isInsideRoot(rootPath, resolved)) {
      return null;
    }
    const targetInfo = await stat(resolved);
    if (!targetInfo.isFile()) {
      return null;
    }
    return { path: resolved, size: targetInfo.size };
  } catch {
    return null;
  }
}

function isInsideRoot(root, target) {
  const relative = path.relative(root, target);
  return relative === "" || (!relative.startsWith("..") && !path.isAbsolute(relative));
}

function commandAliasLocation(parsed) {
  const match = parsed.pathname.match(/^\/wiki\/([A-Za-z0-9_-]+)(?:\.html)?\/?$/);
  if (!match) {
    return "";
  }
  return `/wiki/features/commands/${match[1]}.html${parsed.search}`;
}

async function writeFile(response, method, file) {
  const extension = path.extname(file.path).toLowerCase();
  const cacheControl = extension === ".html" ? "no-cache" : "public, max-age=300, stale-while-revalidate=60";
  response.writeHead(200, responseHeaders({
    "Cache-Control": cacheControl,
    "Content-Length": String(file.size),
    "Content-Type": contentTypes.get(extension) || "application/octet-stream",
  }));
  if (method === "HEAD") {
    response.end();
    return;
  }

  await new Promise((resolve, reject) => {
    const stream = createReadStream(file.path);
    const cleanup = () => {
      response.off("close", onClose);
      response.off("finish", onFinish);
      stream.off("error", onError);
    };
    const onFinish = () => {
      cleanup();
      resolve();
    };
    const onClose = () => {
      stream.destroy();
      onFinish();
    };
    const onError = (error) => {
      cleanup();
      reject(error);
    };
    stream.once("error", onError);
    response.once("close", onClose);
    response.once("finish", onFinish);
    stream.pipe(response);
  });
}

function writeText(response, method, status, body, headers = {}) {
  const content = Buffer.from(`${body}\n`, "utf8");
  response.writeHead(status, responseHeaders({
    "Cache-Control": "no-store",
    "Content-Length": String(content.length),
    "Content-Type": "text/plain; charset=utf-8",
    ...headers,
  }));
  response.end(method === "HEAD" ? undefined : content);
}

function writeRedirect(response, method, location) {
  response.writeHead(302, responseHeaders({
    "Cache-Control": "no-store",
    "Content-Length": "0",
    Location: location,
  }));
  response.end(method === "HEAD" ? undefined : "");
}

function responseHeaders(headers) {
  return { ...securityHeaders, ...headers };
}
