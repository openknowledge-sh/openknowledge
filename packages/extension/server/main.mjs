import { createServer } from 'node:http';
import { existsSync, realpathSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { StreamableHTTPServerTransport } from '@modelcontextprotocol/sdk/server/streamableHttp.js';
import { isInitializeRequest } from '@modelcontextprotocol/sdk/types.js';
import { normalizeViewerURL, startKnowledge } from './knowledge.mjs';
import { createMCPServer } from './mcp.mjs';

export const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export const repoRoot = path.resolve(packageRoot, '../..');

export function options(args = process.argv.slice(2)) {
  const bundledBinary = path.join(packageRoot, 'bin', process.platform === 'win32' ? 'openknowledge.exe' : 'openknowledge');
  const config = { binary: process.env.OPENKNOWLEDGE_BIN || (existsSync(bundledBinary) ? bundledBinary : undefined), repoRoot, port: 4318, stdio: false };
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--') continue;
    if (args[i] === '--stdio') config.stdio = true;
    else if (args[i] === '--root' && args[i + 1]) config.root = path.resolve(args[++i]);
    else if (args[i] === '--viewer-url' && args[i + 1]) config.viewerURL = normalizeViewerURL(args[++i]);
    else if (args[i] === '--port' && args[i + 1]) config.port = Number(args[++i]);
    else throw new Error(`Unknown or incomplete argument: ${args[i]}`);
  }
  if (!Number.isInteger(config.port) || config.port < 0 || config.port > 65535) throw new Error('Invalid port.');
  return config;
}

export function localRequestAllowed(request) {
  // Loopback bind plus Host/Origin checks prevent another website from using
  // this unauthenticated development server to read local knowledge.
  const host = request.headers.host || '';
  if (!/^(127\.0\.0\.1|localhost)(:\d+)?$/.test(host)) return false;
  const origin = request.headers.origin;
  return !origin || origin === `http://${host}`;
}

export async function readJSON(request) {
  const parts = [];
  let size = 0;
  for await (const part of request) {
    size += part.length;
    if (size > 1024 * 1024) throw new Error('Request body exceeds 1 MiB.');
    parts.push(part);
  }
  return JSON.parse(Buffer.concat(parts).toString());
}

export function createMCPHandler(knowledge, htmlPath) {
  const sessions = new Map();
  return {
    async handle(request, response) {
      if (!localRequestAllowed(request)) { response.writeHead(403).end('Local requests only.'); return; }
      try {
        const body = request.method === 'POST' ? await readJSON(request) : undefined;
        const sessionId = request.headers['mcp-session-id'];
        let session = sessions.get(sessionId);
        if (!session && !sessionId && request.method === 'POST' && isInitializeRequest(body)) {
          if (sessions.size >= 32) { response.writeHead(503).end('Too many sessions.'); return; }
          const server = createMCPServer(knowledge, htmlPath);
          const transport = new StreamableHTTPServerTransport({ sessionIdGenerator: randomUUID, onsessioninitialized(id) { sessions.set(id, { server, transport }); } });
          transport.onclose = () => sessions.delete(transport.sessionId);
          await server.connect(transport);
          session = { server, transport };
        }
        if (!session) { response.writeHead(sessionId ? 404 : 400).end('Initialize an MCP session first.'); return; }
        await session.transport.handleRequest(request, response, body);
      } catch (error) {
        if (!response.headersSent) response.writeHead(400, { 'Content-Type': 'application/json' }).end(JSON.stringify({ error: error.message }));
        else response.end();
      }
    },
    async close() { await Promise.all([...sessions.values()].map(({ server }) => server.close())); },
  };
}

async function main() {
  const config = options();
  const backend = await startKnowledge(config);
  const htmlPath = path.join(packageRoot, 'dist/app.html');
  let stop;
  if (config.stdio) {
    const server = createMCPServer(backend.knowledge, htmlPath);
    await server.connect(new StdioServerTransport());
    stop = async () => { await server.close(); await backend.close(); };
    process.stdin.once('end', () => void stop());
  } else {
    const mcp = createMCPHandler(backend.knowledge, htmlPath);
    const http = createServer((req, res) => {
      if (req.url === '/mcp') void mcp.handle(req, res);
      else res.writeHead(404).end();
    });
    http.listen(config.port, '127.0.0.1', () => console.error(`Open Knowledge MCP: http://127.0.0.1:${http.address().port}/mcp`));
    stop = async () => { http.close(); await mcp.close(); await backend.close(); };
  }
  for (const signal of ['SIGINT', 'SIGTERM']) process.once(signal, () => void stop().then(() => process.exit(0)));
}

function isDirectEntry() {
  try { return Boolean(process.argv[1]) && realpathSync(process.argv[1]) === realpathSync(fileURLToPath(import.meta.url)); }
  catch { return false; }
}

if (isDirectEntry()) {
  main().catch(error => { console.error(error); process.exitCode = 1; });
}
