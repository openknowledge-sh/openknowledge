import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createNativeRuntime, nativeViewerURL, viewerRoute, responseFromViewer } from '../web/native-runtime.mjs';

const viewerURL = 'http://127.0.0.1:5000/kb/alpha/file/guides/note.md';
function harness(callTool = async () => ({ status: 200, headers: { 'content-type': 'application/json' }, body: '{"ok":true}', encoding: 'text' })) {
  const document = {};
  Object.defineProperty(document, 'cookie', { get() { throw new DOMException('Opaque origin', 'SecurityError'); }, set() { throw new DOMException('Opaque origin', 'SecurityError'); } });
  const events = [];
  const navigations = [];
  const windowObject = { document, Request, Response, atob, btoa, dispatchEvent(event) { events.push(event); },
    PopStateEvent: class extends Event { constructor(type, options) { super(type); this.state = options.state; } },
    history: { pushState() { assert.fail('Native host history must remain untouched.'); } },
    addEventListener() { assert.equal(this, windowObject); },
  };
  for (const name of ['localStorage', 'sessionStorage']) Object.defineProperty(windowObject, name, { get() { throw new DOMException('Opaque origin', 'SecurityError'); } });
  const runtime = createNativeRuntime({ windowObject, bootstrap: { viewerURL, basePath: '/' }, callTool, onNavigate: (...args) => navigations.push(args) });
  return { runtime, windowObject, document, events, navigations };
}

test('native viewer routes stay on their loopback backend and reject ambiguous paths', () => {
  for (const value of ['https://example.com/', 'http://127.0.0.1.evil/', 'http://user@127.0.0.1/']) assert.throws(() => nativeViewerURL(value));
  assert.equal(viewerRoute('/kb/alpha/api/document?path=guides%2Fnote.md', viewerURL), '/kb/alpha/api/document?path=guides%2Fnote.md');
  for (const value of ['http://127.0.0.1:5001/api/document', 'https://example.com/api/document', '/raw/%2e%2e/secret.png', '/raw/foo%2fbar.png', '/raw/foo\\bar.png']) assert.throws(() => viewerRoute(value, viewerURL));
});

test('virtual location and history retain SPA state without touching opaque-origin host history', () => {
  const { runtime, events, navigations, windowObject } = harness();
  assert.equal(runtime.window.location.href, viewerURL);
  runtime.history.replaceState({ stack: ['first'] }, '', viewerURL);
  runtime.history.pushState({ stack: ['second'] }, '', '/kb/beta/file/index.md?view=graph');
  assert.equal(runtime.location.pathname, '/kb/beta/file/index.md');
  assert.equal(runtime.location.search, '?view=graph');
  assert.equal(runtime.history.length, 2);
  runtime.history.back();
  assert.equal(runtime.location.href, viewerURL);
  assert.deepEqual(events[0].state, { stack: ['first'] });
  runtime.history.forward();
  assert.equal(runtime.location.pathname, '/kb/beta/file/index.md');
  runtime.location.assign('/kb/alpha/file/new.md');
  assert.equal(navigations.at(-1)[1].requested, true);
  assert.equal(runtime.location.pathname, '/kb/beta/file/index.md', 'actual navigation is committed by the viewer SPA');
  runtime.window.addEventListener('ready', () => {});
  assert.equal(runtime.window.Response, windowObject.Response, 'browser constructors retain their prototypes');
});

test('opaque-origin storage and cookies preserve viewer state in memory', () => {
  const { runtime } = harness();
  runtime.localStorage.setItem('theme', 'night');
  runtime.sessionStorage.setItem('draft', 'Unsaved text');
  assert.equal(runtime.window.localStorage.getItem('theme'), 'night');
  assert.equal(runtime.sessionStorage.getItem('draft'), 'Unsaved text');
  assert.equal(runtime.localStorage.getItem('draft'), null);
  runtime.document.cookie = 'ok-theme=night; path=/';
  assert.equal(runtime.document.cookie, 'ok-theme=night');
});

test('native fetch delegates exact GET and optimistic save JSON to scoped MCP tools', async () => {
  const calls = [];
  const { runtime } = harness(async (name, args, signal) => { calls.push({ name, args, signal }); return { status: name.endsWith('save') ? 409 : 200, headers: { 'content-type': 'application/json' }, body: '{"revision":"source","conflict":true}', encoding: 'text' }; });
  const read = await runtime.fetch('/kb/alpha/api/document?path=guides%2Fnote.md');
  assert.equal(read.ok, true);
  assert.equal((await read.json()).revision, 'source');
  const body = JSON.stringify({ path: 'guides/note.md', revision: 'source', text: '![Image](../media/pixel.png)' });
  const save = await runtime.fetch('/kb/alpha/api/document?path=guides%2Fnote.md', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body });
  assert.equal(save.status, 409);
  assert.deepEqual(calls[1].args, { route: '/kb/alpha/api/document?path=guides%2Fnote.md', body });
  assert.equal(calls[1].name, 'openknowledge_viewer_save');
  const unsupported = await runtime.fetch('/api/knowledge-bases', { method: 'POST', body: '{}' });
  assert.equal(unsupported.status, 405);
  assert.equal(calls.length, 2);
});

test('native fetch preserves abort and read-only/error status semantics', async () => {
  const controller = new AbortController();
  const { runtime } = harness(async (name, args, signal) => {
    assert.ok(signal);
    controller.abort(new DOMException('Cancelled', 'AbortError'));
    return { status: 403, headers: { 'content-type': 'application/json' }, body: '{"error":"read-only"}', encoding: 'text' };
  });
  await assert.rejects(runtime.fetch('/api/document', { signal: controller.signal }), { name: 'AbortError' });
  const readonly = responseFromViewer({ status: 403, headers: { 'content-type': 'application/json' }, body: '{"error":"read-only"}', encoding: 'text' });
  assert.equal(readonly.ok, false);
  assert.equal((await readonly.json()).error, 'read-only');
});

test('relative rich images use native raw assets and embedded data without changing Markdown', async () => {
  const calls = [];
  const { runtime } = harness(async (name, args) => { calls.push({ name, args }); return { status: 200, headers: { 'content-type': 'image/png' }, body: 'iVBORw0KGgo=', encoding: 'base64' }; });
  const original = '../media/pixel.png';
  const embedded = await runtime.assetURL(original);
  assert.equal(embedded, 'data:image/png;base64,iVBORw0KGgo=');
  assert.equal(calls[0].args.route, '/kb/alpha/raw/media/pixel.png');
  assert.equal(original, '../media/pixel.png');
  assert.equal(await runtime.assetURL(original), embedded);
  assert.equal(calls.length, 1, 'assets are shared across repeated renders');
  await assert.rejects(runtime.assetURL('https://example.com/remote.png'), /Remote assets/);
});
