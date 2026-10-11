import { createServer } from 'vite';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { buildApp } from './build.mjs';
import { startKnowledge } from '../server/knowledge.mjs';
import { callKnowledgeTool, prepareAppResource } from '../server/mcp.mjs';
import { prepareNativeViewerResource } from '../server/native-viewer.mjs';
import { options, packageRoot, localRequestAllowed, readJSON, createMCPHandler } from '../server/main.mjs';

const config = options();
const backend = await startKnowledge(config);
let vite;
const mcp = createMCPHandler(backend.knowledge, path.join(packageRoot, 'dist/app.html'));
try {
  await buildApp();
  vite = await createServer({
    configFile: false, root: path.join(packageRoot, 'web'),
    server: { host: '127.0.0.1', port: config.port, strictPort: true },
    plugins: [{
      name: 'openknowledge-extension-dev',
      configureServer(server) {
        server.middlewares.use(async (request, response, next) => {
          if (!localRequestAllowed(request)) { response.writeHead(403).end('Local requests only.'); return; }
          if (request.url === '/mcp') { await mcp.handle(request, response); return; }
          if (!['/api/tool', '/api/test-app'].includes(request.url)) { next(); return; }
          response.setHeader('Content-Type', 'application/json');
          response.setHeader('Cache-Control', 'no-store');
          if (request.method !== 'POST') { response.writeHead(405).end(); return; }
          try {
            if (request.url === '/api/test-app') {
              if (request.headers.origin !== `http://${request.headers.host}` || request.headers['content-type']?.split(';')[0].trim() !== 'application/json') {
                response.writeHead(403).end(JSON.stringify({ error: 'A same-origin JSON request is required.' }));
                return;
              }
              const input = await readJSON(request);
              let prepared;
              if (input.native) {
                const adapter = await readFile(path.join(packageRoot, 'dist/native-viewer.js'), 'utf8');
                prepared = await prepareNativeViewerResource(backend.knowledge, adapter);
              } else {
                const html = await readFile(path.join(packageRoot, 'dist/app.html'), 'utf8');
                prepared = await prepareAppResource(backend.knowledge, html);
              }
              response.end(JSON.stringify({ url: await backend.knowledge.getTestAppURL(prepared.html), csp: prepared.csp || { connectDomains: [], resourceDomains: [], frameDomains: prepared.frameDomains } }));
              return;
            }
            const { name, arguments: args } = await readJSON(request);
            response.end(JSON.stringify(await callKnowledgeTool(backend.knowledge, name, args)));
          } catch (error) { response.writeHead(400).end(JSON.stringify({ error: error.message })); }
        });
      },
    }],
  });
  await vite.listen();
  // Vite updates the browser preview immediately. Rebuild the self-contained
  // MCP resource as well, ready for the host's next resources/read request.
  let building = false;
  let dirty = false;
  const rebuild = async () => {
    if (building) { dirty = true; return; }
    building = true;
    do {
      dirty = false;
      try { await buildApp(); } catch (error) { console.error('UI build failed:', error.message); }
    } while (dirty);
    building = false;
  };
  vite.watcher.on('change', file => {
    if (file.startsWith(path.join(packageRoot, 'web'))) void rebuild();
  });
  console.log(`Open Knowledge extension preview: http://127.0.0.1:${config.port}/`);
  console.log(`MCP endpoint: http://127.0.0.1:${config.port}/mcp`);
  console.log('UI: Vite live updates. MCP metadata/resources: refresh the host connection.');
} catch (error) {
  await vite?.close();
  await backend.close();
  throw error;
}
let stopping = false;
async function stop() {
  if (stopping) return;
  stopping = true;
  await mcp.close();
  await vite.close();
  await backend.close();
}
for (const signal of ['SIGINT', 'SIGTERM']) process.once(signal, () => void stop().then(() => process.exit(0)));
