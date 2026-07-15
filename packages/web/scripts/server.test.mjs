import assert from "node:assert/strict";
import { mkdtemp, mkdir, rm, symlink, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { after, before, test } from "node:test";
import { Writable } from "node:stream";
import { createWebHandler, createWebServer } from "./server.mjs";

let handler;
let root;
let server;
let temporary;

before(async () => {
  temporary = await mkdtemp(path.join(os.tmpdir(), "openknowledge-web-test-"));
  root = path.join(temporary, "public");
  const fallback = path.join(temporary, "fallback");
  await mkdir(path.join(root, "guide"), { recursive: true });
  await mkdir(path.join(fallback, "wiki"), { recursive: true });
  await writeFile(path.join(root, "index.html"), "<!doctype html><title>Home</title>");
  await writeFile(path.join(root, "main.js"), "console.log('ok');\n");
  await writeFile(path.join(root, "guide", "index.html"), "<!doctype html><title>Guide</title>");
  await writeFile(path.join(fallback, "wiki", "fallback.html"), "<!doctype html><title>Fallback</title>");
  const outside = path.join(temporary, "outside.txt");
  await writeFile(outside, "private\n");
  await symlink(outside, path.join(root, "leak.txt"));

  handler = createWebHandler({ root, fallbackRoot: fallback });
  server = createWebServer({ root, fallbackRoot: fallback });
});

after(async () => {
  server?.close();
  if (temporary) {
    await rm(temporary, { recursive: true, force: true });
  }
});

test("serves files and directories with security and cache headers", async () => {
  const response = await request("/");
  assert.equal(response.statusCode, 200);
  assert.match(response.body(), /<title>Home<\/title>/);
  assert.equal(response.header("content-type"), "text/html; charset=utf-8");
  assert.equal(response.header("cache-control"), "no-cache");
  assertSecurityHeaders(response);

  const asset = await request("/main.js");
  assert.equal(asset.statusCode, 200);
  assert.equal(asset.header("content-type"), "text/javascript; charset=utf-8");
  assert.equal(asset.header("cache-control"), "public, max-age=300, stale-while-revalidate=60");

  const directory = await request("/guide/");
  assert.equal(directory.statusCode, 200);
  assert.match(directory.body(), /<title>Guide<\/title>/);
});

test("implements HEAD without a response body", async () => {
  const response = await request("/main.js", "HEAD");
  assert.equal(response.statusCode, 200);
  assert.equal(response.body(), "");
  assert.equal(response.header("content-length"), String(Buffer.byteLength("console.log('ok');\n")));
  assertSecurityHeaders(response);
});

test("rejects unsupported methods and malformed URLs", async () => {
  const method = await request("/", "POST");
  assert.equal(method.statusCode, 405);
  assert.equal(method.header("allow"), "GET, HEAD");
  assert.equal(method.header("cache-control"), "no-store");
  assertSecurityHeaders(method);

  const malformed = await request("/%E0%A4%A");
  assert.equal(malformed.statusCode, 400);
  assert.equal(malformed.body(), "Bad request\n");
  assertSecurityHeaders(malformed);
});

test("keeps redirects explicit and uncached", async () => {
  const install = await request("/install");
  assert.equal(install.statusCode, 302);
  assert.equal(install.header("location"), "https://github.com/openknowledge-sh/openknowledge/releases/latest/download/install");
  assert.equal(install.header("cache-control"), "no-store");
  assertSecurityHeaders(install);

  const alias = await request("/wiki/setup?source=test");
  assert.equal(alias.statusCode, 302);
  assert.equal(alias.header("location"), "/wiki/features/commands/setup.html?source=test");
});

test("serves configured wiki fallback but rejects symlink escapes", async () => {
  const fallback = await request("/wiki/fallback.html");
  assert.equal(fallback.statusCode, 200);
  assert.match(fallback.body(), /<title>Fallback<\/title>/);

  const leak = await request("/leak.txt");
  assert.equal(leak.statusCode, 404);
  assert.equal(leak.body(), "Not found\n");
  assertSecurityHeaders(leak);
});

test("bounds HTTP server resources", () => {
  assert.equal(server.maxHeaderSize, 16 * 1024);
  assert.equal(server.headersTimeout, 10_000);
  assert.equal(server.requestTimeout, 15_000);
  assert.equal(server.keepAliveTimeout, 5_000);
  assert.equal(server.maxRequestsPerSocket, 100);
});

async function request(url, method = "GET") {
  const response = new MemoryResponse();
  await handler({ method, url, resume() {} }, response);
  return response;
}

class MemoryResponse extends Writable {
  constructor() {
    super();
    this.headersSent = false;
    this.statusCode = 0;
    this.headers = new Map();
    this.chunks = [];
  }

  _write(chunk, _encoding, callback) {
    this.chunks.push(Buffer.from(chunk));
    callback();
  }

  writeHead(status, headers) {
    this.statusCode = status;
    this.headersSent = true;
    for (const [name, value] of Object.entries(headers)) {
      this.headers.set(name.toLowerCase(), String(value));
    }
    return this;
  }

  header(name) {
    return this.headers.get(name.toLowerCase()) || null;
  }

  body() {
    return Buffer.concat(this.chunks).toString("utf8");
  }
}

function assertSecurityHeaders(response) {
  assert.equal(response.header("x-content-type-options"), "nosniff");
  assert.equal(response.header("x-frame-options"), "DENY");
  assert.equal(response.header("referrer-policy"), "strict-origin-when-cross-origin");
  assert.equal(response.header("cross-origin-opener-policy"), "same-origin");
  assert.equal(response.header("strict-transport-security"), "max-age=31536000");
  assert.match(response.header("content-security-policy"), /frame-ancestors 'none'/);
  assert.match(response.header("permissions-policy"), /camera=\(\)/);
}
