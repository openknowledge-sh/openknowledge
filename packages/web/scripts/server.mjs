import { createReadStream } from "node:fs";
import { realpath, stat } from "node:fs/promises";
import { randomUUID } from "node:crypto";
import http from "node:http";
import path from "node:path";

const maxTelemetryBytes = 16 * 1024;
const telemetryEventNames = new Set([
  "web_page_viewed",
  "setup_prompt_copied",
  "install_redirect_requested",
  "cli_command_completed",
  "cli_first_command",
  "cli_setup_completed",
  "cli_first_meaningful_use",
  "cli_daily_active",
  "cli_error",
]);
const cliTelemetryEventNames = new Set([
  "cli_command_completed", "cli_first_command", "cli_setup_completed",
  "cli_first_meaningful_use", "cli_daily_active", "cli_error",
]);
const webTelemetryEventNames = new Set(["web_page_viewed", "setup_prompt_copied"]);
const telemetryFields = new Set([
  "schema_version", "event_name", "event_id", "occurred_at", "surface",
  "installation_id", "anonymous_id", "app_version", "os", "arch", "command",
  "outcome", "duration_bucket", "error_kind", "page_group", "interaction",
  "source", "client_family",
]);
const commonTelemetryFields = ["schema_version", "event_name", "event_id", "occurred_at", "surface"];
const cliTelemetryFields = new Set([
  ...commonTelemetryFields, "installation_id", "app_version", "os", "arch", "command",
  "outcome", "duration_bucket", "error_kind",
]);
const webTelemetryFields = new Set([...commonTelemetryFields, "anonymous_id", "page_group", "interaction"]);
const serverTelemetryFields = new Set([...commonTelemetryFields, "source", "client_family"]);
const postHogEventPropertyFields = new Set([
  "app_version", "os", "arch", "command", "outcome", "duration_bucket", "error_kind",
  "page_group", "interaction", "source", "client_family",
]);
const telemetryCommands = new Set([
  "openknowledge", "setup", "setup skill", "setup complete", "setup status", "setup repair", "setup observe",
  "search", "validate", "agent", "agent exec", "agent doctor", "get", "list", "view",
  "export", "export html", "export json", "export tar", "export graph", "mcp", "connect", "disconnect",
  "registry", "registry refresh", "registry list", "registry status", "registry where",
  "automation", "automation jobs", "automation insights", "automation runtime", "automation deploy",
  "scaffold", "prompt", "prompt rules", "prompt review", "ast", "spec", "version",
]);

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
  telemetryUpstream = "",
  telemetryToken = "",
  telemetrySink,
}) {
  if (!root) {
    throw new Error("web root is required");
  }

  return async (request, response) => {
    try {
      await handleRequest(request, response, {
        root, fallbackRoot, installLocation, telemetryUpstream, telemetryToken, telemetrySink,
      });
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
  const parsed = parseRequestURL(request.url || "/");
  if (!parsed) {
    writeText(response, method, 400, "Bad request");
    return;
  }

  if (method === "POST" && parsed.pathname === "/api/telemetry") {
    await receiveTelemetry(request, response, options);
    return;
  }
  if (method !== "GET" && method !== "HEAD") {
    request.resume();
    writeText(response, method, 405, "Method not allowed", { Allow: "GET, HEAD" });
    return;
  }

  if (parsed.pathname === "/install" || parsed.pathname === "/install/") {
    if (method === "GET") {
      queueTelemetry(options, {
        schema_version: "1",
        event_name: "install_redirect_requested",
        event_id: randomUUID(),
        occurred_at: new Date().toISOString(),
        surface: "server",
        source: installSource(parsed.search),
        client_family: clientFamily(request.headers?.["user-agent"]),
      });
    }
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

async function receiveTelemetry(request, response, options) {
  const contentType = String(request.headers?.["content-type"] || "").toLowerCase();
  if (!contentType.startsWith("application/json")) {
    request.resume();
    writeText(response, request.method, 415, "Unsupported media type");
    return;
  }
  let envelope;
  try {
    envelope = JSON.parse(await readBodyAtMost(request, maxTelemetryBytes));
  } catch (error) {
    const status = error?.code === "TOO_LARGE" ? 413 : 400;
    writeText(response, request.method, status, status === 413 ? "Payload too large" : "Invalid telemetry payload");
    return;
  }
  if (!validTelemetryEnvelope(envelope)) {
    writeText(response, request.method, 400, "Invalid telemetry payload");
    return;
  }
  writeEmpty(response, 204);
  queueTelemetry(options, envelope);
}

async function readBodyAtMost(request, limit) {
  const chunks = [];
  let received = 0;
  for await (const chunk of request) {
    const content = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    received += content.length;
    if (received > limit) {
      const error = new Error("telemetry payload is too large");
      error.code = "TOO_LARGE";
      throw error;
    }
    chunks.push(content);
  }
  return Buffer.concat(chunks).toString("utf8");
}

function validTelemetryEnvelope(envelope) {
  if (!plainObject(envelope) || envelope.schema_version !== "1" || !Array.isArray(envelope.events)) return false;
  if (Object.keys(envelope).some((key) => key !== "schema_version" && key !== "events")) return false;
  if (envelope.events.length < 1 || envelope.events.length > 8) return false;
  return envelope.events.every(validTelemetryEvent);
}

function validTelemetryEvent(event) {
  if (!plainObject(event) || Object.keys(event).some((key) => !telemetryFields.has(key))) return false;
  if (event.schema_version !== "1" || !telemetryEventNames.has(event.event_name)) return false;
  if (!safeToken(event.event_id, 128) || typeof event.occurred_at !== "string" ||
      event.occurred_at.length > 40 || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$/.test(event.occurred_at) ||
      Number.isNaN(Date.parse(event.occurred_at))) return false;
  if (!["cli", "web", "server"].includes(event.surface)) return false;
  for (const [key, value] of Object.entries(event)) {
    if (typeof value !== "string" || value.length > 160) return false;
    if (key !== "occurred_at" && /[\r\n]/.test(value)) return false;
  }
  if (event.surface === "cli") {
    return Object.keys(event).every((key) => cliTelemetryFields.has(key)) &&
      cliTelemetryEventNames.has(event.event_name) && safeToken(event.installation_id, 128) && safeToken(event.app_version, 64) &&
      safeToken(event.os, 32) && safeToken(event.arch, 32) && telemetryCommands.has(event.command) &&
      (!event.outcome || ["success", "error"].includes(event.outcome)) &&
      (!event.duration_bucket || ["under-10ms", "10-100ms", "100ms-1s", "1-10s", "10s-or-more"].includes(event.duration_bucket)) &&
      (!event.error_kind || ["usage", "command_failed"].includes(event.error_kind));
  }
  if (event.surface === "web") {
    return Object.keys(event).every((key) => webTelemetryFields.has(key)) &&
      webTelemetryEventNames.has(event.event_name) && safeToken(event.anonymous_id, 128) && event.page_group === "home" &&
      (!event.interaction || event.interaction === "setup_prompt");
  }
  return Object.keys(event).every((key) => serverTelemetryFields.has(key)) && event.event_name === "install_redirect_requested" &&
    ["homepage", "docs", "readme", "direct", "other"].includes(event.source) &&
    ["curl", "wget", "browser", "other"].includes(event.client_family);
}

function plainObject(value) {
  return Boolean(value && typeof value === "object" && !Array.isArray(value) && Object.getPrototypeOf(value) === Object.prototype);
}

function safeToken(value, max) {
  return typeof value === "string" && value.length > 0 && value.length <= max && /^[A-Za-z0-9._:-]+$/.test(value);
}

function queueTelemetry(options, value) {
  const envelope = value.events ? value : { schema_version: "1", events: [value] };
  const delivery = options.telemetrySink
    ? options.telemetrySink(envelope)
    : forwardTelemetry(envelope, options.telemetryUpstream, options.telemetryToken);
  if (delivery && typeof delivery.catch === "function") delivery.catch(() => {});
}

export function toPostHogBatch(envelope, projectToken) {
  return {
    api_key: projectToken,
    historical_migration: false,
    batch: envelope.events.flatMap((event) => {
      const productEvent = {
        event: event.event_name,
        timestamp: event.occurred_at,
        properties: postHogBaseProperties(event),
      };
      if (event.event_name !== "cli_error") return [productEvent];
      return [productEvent, sanitizedPostHogException(event)];
    }),
  };
}

function postHogBaseProperties(event) {
  return {
    distinct_id: postHogDistinctID(event),
    $process_person_profile: false,
    schema_version: event.schema_version,
    event_id: event.event_id,
    surface: event.surface,
    ...postHogEventProperties(event),
  };
}

function sanitizedPostHogException(event) {
  const usageError = event.error_kind === "usage";
  return {
    event: "$exception",
    timestamp: event.occurred_at,
    properties: {
      ...postHogBaseProperties(event),
      $exception_list: [{
        type: usageError ? "OpenKnowledgeUsageError" : "OpenKnowledgeCommandError",
        value: usageError ? "The command returned a usage error." : "The command failed.",
        mechanism: { handled: true, synthetic: true },
      }],
      $exception_fingerprint: `openknowledge-cli:${event.command}:${event.error_kind}`,
      $exception_level: "error",
    },
  };
}

export async function forwardTelemetry(envelope, upstream, projectToken, fetchImplementation = fetch) {
  if (!upstream || !projectToken) return;
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 2000);
  try {
    const response = await fetchImplementation(postHogBatchEndpoint(upstream), {
      method: "POST",
      headers: { "Content-Type": "application/json", "User-Agent": "openknowledge-telemetry-relay/1" },
      body: JSON.stringify(toPostHogBatch(envelope, projectToken)),
      signal: controller.signal,
    });
    if (!response.ok) throw new Error(`PostHog ingestion failed with status ${response.status}`);
  } finally {
    clearTimeout(timeout);
  }
}

function postHogBatchEndpoint(upstream) {
  const endpoint = new URL(upstream);
  if (endpoint.protocol !== "https:" && endpoint.protocol !== "http:") {
    throw new Error("PostHog telemetry upstream must use HTTP or HTTPS");
  }
  if (endpoint.username || endpoint.password || endpoint.search || endpoint.hash) {
    throw new Error("PostHog telemetry upstream must not contain credentials, a query, or a fragment");
  }
  if (endpoint.pathname === "/") endpoint.pathname = "/batch/";
  if (endpoint.pathname !== "/batch" && endpoint.pathname !== "/batch/") {
    throw new Error("PostHog telemetry upstream must be a PostHog batch endpoint");
  }
  endpoint.pathname = "/batch/";
  return endpoint.toString();
}

function postHogDistinctID(event) {
  if (event.surface === "cli") return `cli:${event.installation_id}`;
  if (event.surface === "web") return `web:${event.anonymous_id}`;
  return `server:${event.event_id}`;
}

function postHogEventProperties(event) {
  return Object.fromEntries(Object.entries(event).filter(([key]) => postHogEventPropertyFields.has(key)));
}

function installSource(search) {
  const source = new URLSearchParams(search).get("source") || "direct";
  return ["homepage", "docs", "readme", "direct"].includes(source) ? source : "other";
}

function clientFamily(userAgent) {
  const value = String(userAgent || "").toLowerCase();
  if (value.includes("curl")) return "curl";
  if (value.includes("wget")) return "wget";
  if (value.includes("mozilla")) return "browser";
  return "other";
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

function writeEmpty(response, status) {
  response.writeHead(status, responseHeaders({
    "Cache-Control": "no-store",
    "Content-Length": "0",
  }));
  response.end();
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
