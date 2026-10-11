import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';
import { AppBridge, PostMessageTransport } from '@modelcontextprotocol/ext-apps/app-bridge';

const client = new Client({ name: 'Open Knowledge development host', version: '0.0.1' });
const transport = new StreamableHTTPClientTransport(new URL('/mcp', window.location.href));
let bridge;
const compatibilityProbe = new URLSearchParams(window.location.search).get('compatibility') === 'omitted-csp-split-tools';
const nativeMode = new URLSearchParams(window.location.search).get('mode')?.startsWith('native');
const opaqueMode = new URLSearchParams(window.location.search).get('mode') === 'native-opaque';
// Keep the development-only context inspector clear of the native controls.
if (nativeMode) {
  const inspector = document.getElementById('context').closest('details');
  inspector.style.left = 'auto';
  inspector.style.right = '20px';
}
function toolResultForHost(result) {
  if (!compatibilityProbe || !result.structuredContent) return result;
  // Simulate another MCP pool's local addresses while retaining its data.
  const data = { ...result.structuredContent };
  for (const key of ['viewerURL', 'editorURL', 'panelURL', 'panelEditorURL']) {
    if (!data[key]) continue;
    const url = new URL(data[key]);
    url.port = key.startsWith('panel') ? '2' : '1';
    data[key] = url.href;
  }
  return { ...result, structuredContent: data };
}
async function load() {
  await bridge?.close();
  const query = new URLSearchParams(window.location.search);
  const requestedBase = query.get('knowledgeBase');
  const requestedPath = query.get('path');
  const result = toolResultForHost(await client.callTool({ name: 'openknowledge_library', arguments: requestedBase ? { knowledgeBase: requestedBase, ...(requestedPath ? { path: requestedPath } : {}) } : {} }));
  const appResponse = await fetch('/api/test-app', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ native: nativeMode }) });
  const app = await appResponse.json();
  if (!appResponse.ok) throw new Error(app.error || 'Could not create an isolated test app.');
  const appURL = new URL(app.url);
  const csp = app.csp;
  if (appURL.origin === window.location.origin || appURL.protocol !== 'http:' || appURL.hostname !== '127.0.0.1' || !csp || (!nativeMode && !csp.frameDomains?.includes(appURL.origin))) {
    throw new Error('The development app must run on a separate approved loopback origin.');
  }
  const frame = document.getElementById('mcp-app');
  bridge = new AppBridge(client, { name: 'Open Knowledge development host', version: '0.0.1' }, { serverTools: {}, updateModelContext: {}, openLinks: {}, logging: {}, ...(compatibilityProbe ? {} : { sandbox: { csp } }) }, { hostContext: { theme: 'light', displayMode: 'fullscreen', availableDisplayModes: ['inline', 'fullscreen'], ...(query.get('deepLink') ? { 'openai/deepLink': { url: query.get('deepLink') } } : {}) } });
  if (compatibilityProbe) bridge.oncalltool = async params => toolResultForHost(await client.callTool(params));
  bridge.onupdatemodelcontext = async context => {
    document.getElementById('context').textContent = JSON.stringify(context, null, 2);
    document.getElementById('status').textContent = 'Context received · no model connected';
    return {};
  };
  bridge.onopenlink = async ({ url }) => {
    if (!/^https?:\/\//.test(url)) throw new Error('Only HTTP links are supported.');
    window.open(url, '_blank', 'noopener,noreferrer');
    return {};
  };
  bridge.oninitialized = async () => {
    await bridge.sendToolInput({ arguments: {} });
    await bridge.sendToolResult(result);
    document.getElementById('status').textContent = 'MCP Apps bridge connected';
  };
  await bridge.connect(new PostMessageTransport(frame.contentWindow, frame.contentWindow));
  // Separate origins isolate host DOM/storage while preserving the real local
  // viewer's origin for editing. Never grant same-origin to host-origin srcdoc.
  frame.removeAttribute('srcdoc');
  frame.setAttribute('sandbox', opaqueMode ? 'allow-scripts allow-forms' : 'allow-scripts allow-same-origin allow-forms');
  frame.src = appURL.href;
}
const fail = error => { document.getElementById('status').textContent = error.message; console.error(error); };
document.getElementById('reload').onclick = () => void load().catch(fail);
try { await client.connect(transport); await load(); } catch (error) { fail(error); }
window.addEventListener('pagehide', () => { void transport.terminateSession().catch(() => {}); void client.close(); });
