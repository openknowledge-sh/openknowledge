// Verify the installed plugin with Codex's own loader, not only an MCP client.
// This does not install plugins, create chats, or start a model turn.
import { spawn } from 'node:child_process';
import { createInterface } from 'node:readline';
import { readFile } from 'node:fs/promises';
import assert from 'node:assert/strict';

const { version } = JSON.parse(await readFile(new URL('../plugin.json', import.meta.url), 'utf8'));
const child = spawn(process.env.CODEX_BIN || 'codex', ['app-server', '--stdio'], { stdio: ['pipe', 'pipe', 'pipe'] });
const pending = new Map();
let sequence = 0;
let diagnostic = '';
child.stderr.on('data', chunk => { diagnostic = (diagnostic + chunk).slice(-30000); });
child.on('error', error => {
  for (const { reject, timer } of pending.values()) { clearTimeout(timer); reject(error); }
  pending.clear();
});
createInterface({ input: child.stdout }).on('line', line => {
  let message;
  try { message = JSON.parse(line); } catch { return; }
  if (message.id === undefined || !pending.has(message.id)) return;
  const { resolve, reject, timer } = pending.get(message.id);
  pending.delete(message.id);
  clearTimeout(timer);
  message.error ? reject(new Error(message.error.message)) : resolve(message.result);
});
const rpc = (method, params) => new Promise((resolve, reject) => {
  const id = ++sequence;
  const timer = setTimeout(() => { pending.delete(id); reject(new Error(`${method} timed out`)); }, 40000);
  pending.set(id, { resolve, reject, timer });
  child.stdin.write(JSON.stringify({ id, method, params }) + '\n');
});
try {
  await rpc('initialize', { clientInfo: { name: 'okn-installed-plugin-check', version: '1' }, capabilities: { experimentalApi: true } });
  child.stdin.write(JSON.stringify({ method: 'initialized', params: {} }) + '\n');
  const inventory = await rpc('mcpServerStatus/list', { serverName: 'openknowledge', detail: 'full' });
  const server = inventory.data.find(item => item.name === 'openknowledge');
  assert.ok(server, 'Install the local Open Knowledge plugin before this check.');
  assert.equal(server.pluginId, 'openknowledge-local@openknowledge-dev');
  assert.equal(server.serverInfo?.version, version, 'Rebuild and reinstall the current plugin version.');
  assert.equal(server.httpOrigin, null, 'The installed plugin must use stdio.');
  assert.equal(server.toolsError, null);
  assert.equal(Object.keys(server.tools || {}).length, 6);
  const resource = await rpc('mcpServer/resource/read', { server: 'openknowledge', uri: 'ui://openknowledge/library.html' });
  const html = resource.contents?.[0];
  assert.equal(html?.mimeType, 'text/html;profile=mcp-app');
  assert.match(html.text, /data-note-workspace/);
  assert.match(html.text, /OpenKnowledgeNativeRuntime/);
  const markup = html.text.replace(/<script\b[^>]*>[\s\S]*?<\/script\s*>/gi, '').replace(/<style\b[^>]*>[\s\S]*?<\/style\s*>/gi, '');
  assert.ok(!/<script\b[^>]*\bsrc=/i.test(html.text), 'Viewer scripts must be inline.');
  assert.ok(!/<iframe\b/i.test(markup), 'The resource must render its viewer directly.');
  const bootstrapScript = html.text.match(/<script type="application\/json" id="openknowledge-native-viewer-bootstrap">([^<]+)<\/script>/);
  assert.ok(bootstrapScript, 'The UI resource must contain its viewer route scope.');
  const bootstrap = JSON.parse(bootstrapScript[1]);
  assert.match(new URL(bootstrap.viewerURL).origin, /^http:\/\/127\.0\.0\.1:\d+$/);
  assert.deepEqual(html._meta?.ui?.csp?.frameDomains, []);
  assert.deepEqual(html._meta?.ui?.csp?.connectDomains, []);
  console.log(`Codex loader passed: installed plugin ${version}, stdio startup, six tools and full viewer resource with inline assets and no local frames/connections.`);
} catch (error) {
  console.error(error.message);
  for (const line of diagnostic.split('\n').filter(line => /openknowledge/.test(line)).slice(-3)) console.error(line);
  process.exitCode = 1;
} finally {
  child.stdin.end();
  child.kill('SIGTERM');
  for (const item of pending.values()) clearTimeout(item.timer);
}
