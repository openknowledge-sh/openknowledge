import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { prepareNativeHTMLImages, prepareNativeViewerResource, requestNativeViewer, viewerRequestURL, MAX_SAVE_BYTES } from '../server/native-viewer.mjs';

async function fixture(handler) {
  const calls = [];
  const server = createServer(async (request, response) => {
    const body = [];
    for await (const chunk of request) body.push(chunk);
    calls.push({ route: request.url, method: request.method, headers: request.headers, body: Buffer.concat(body).toString() });
    handler(request, response, calls.at(-1));
  });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
  const origin = `http://127.0.0.1:${server.address().port}`;
  return { calls, origin, knowledge: { getViewerHome: async () => ({ viewerURL: origin + '/docs/' }) }, async close() { await new Promise(resolve => { server.close(resolve); server.closeAllConnections(); }); } };
}

test('native viewer routes preserve root authority and reject ambiguous paths and other write endpoints', () => {
  const home = 'http://127.0.0.1:5176/docs/';
  assert.equal(viewerRequestURL(home, '/docs/api/document?path=A%20%26%20B.md').href, home + 'api/document?path=A%20%26%20B.md');
  assert.equal(viewerRequestURL(home, '/assets/openknowledge/viewer.js').pathname, '/assets/openknowledge/viewer.js');
  assert.equal(viewerRequestURL(home, '/docs/api/document', true).pathname, '/docs/api/document');
  assert.equal(viewerRequestURL('http://127.0.0.1:5176/org/wiki/', '/org/wiki/api/document', true).pathname, '/org/wiki/api/document');
  for (const route of ['http://evil.example/docs/', '//evil.example/docs/', '/other/file/note.md', '/docs/../other', '/docs/%2e%2e/other', '/docs/file/a%2fb.md', '/docs/file/a%5cb.md', '/docs/file/a\\b.md', '/docs/#page', '/docs/\n']) assert.throws(() => viewerRequestURL(home, route));
  for (const route of ['/docs/api/feedback', '/docs/api/document/', '/assets/openknowledge/viewer.js', '/docs/file/note.md']) assert.throws(() => viewerRequestURL(home, route, true), /save endpoint|outside/);
  assert.throws(() => viewerRequestURL('https://example.com/', '/'), /local/);
});

test('native transport keeps read and save methods separate and forwards only the fixed local Origin', async () => {
  const runtime = await fixture((request, response, call) => {
    if (request.url === '/docs/raw/media/a.png') { response.writeHead(200, { 'content-type': 'image/png' }).end(Buffer.from([0, 1, 255])); return; }
    response.writeHead(request.method === 'POST' ? 409 : 200, { 'content-type': 'application/json; charset=utf-8' }).end(JSON.stringify({ method: request.method, received: call.body }));
  });
  try {
    const read = await requestNativeViewer(runtime.knowledge, { route: '/docs/api/document?path=note.md' });
    assert.equal(read.encoding, 'text');
    assert.equal(JSON.parse(read.body).method, 'GET');
    const body = '{"path":"note.md","text":"# Note","expectedRevision":"revision"}';
    const saved = await requestNativeViewer(runtime.knowledge, { route: '/docs/api/document', body }, true);
    assert.equal(saved.status, 409, 'native revision conflicts remain visible to the editor');
    assert.equal(JSON.parse(saved.body).received, body);
    assert.equal(runtime.calls.at(-1).headers.origin, runtime.origin);
    assert.equal(runtime.calls.at(-1).headers['content-type'], 'application/json');
    const image = await requestNativeViewer(runtime.knowledge, { route: '/docs/file/media/a.png' });
    assert.equal(image.encoding, 'base64');
    assert.deepEqual(Buffer.from(image.body, image.encoding), Buffer.from([0, 1, 255]));
    assert.equal(runtime.calls.at(-1).route, '/docs/raw/media/a.png');
    const count = runtime.calls.length;
    await assert.rejects(requestNativeViewer(runtime.knowledge, { route: '/docs/api/document', body }, false), /do not accept/);
    await assert.rejects(requestNativeViewer(runtime.knowledge, { route: '/docs/api/feedback', body }, true), /save endpoint/);
    await assert.rejects(requestNativeViewer(runtime.knowledge, { route: '/docs/api/document', body: 'a'.repeat(MAX_SAVE_BYTES + 1) }, true), /allowed size/);
    assert.equal(runtime.calls.length, count);
  } finally { await runtime.close(); }
});

test('direct native resource copies the full viewer, inlines trusted assets and needs no frame or network authority', async () => {
  const runtime = await fixture((request, response) => {
    if (request.url === '/docs/') { response.writeHead(302, { location: '/docs/file/index.md' }).end(); return; }
    if (request.url === '/assets/openknowledge/viewer-theme.js?v=1') { response.writeHead(200, { 'content-type': 'text/javascript' }).end('window.theme = document.cookie;'); return; }
    if (request.url === '/assets/openknowledge/viewer.js?v=1') { response.writeHead(200, { 'content-type': 'text/javascript' }).end('window.viewer = function(){return [location.pathname,history,fetch,localStorage,sessionStorage]};'); return; }
    if (request.url === '/assets/openknowledge/viewer.css?v=1') { response.writeHead(200, { 'content-type': 'text/css' }).end('.file-sidebar { display: block; }'); return; }
    if (request.url === '/docs/raw/theme.css') { response.writeHead(200, { 'content-type': 'text/css' }).end('body { color: green; }'); return; }
    response.writeHead(200, { 'content-type': 'text/html' }).end('<!doctype html><html><head><script src="/assets/openknowledge/viewer-theme.js?v=1"></script><link rel="stylesheet" href="/assets/openknowledge/viewer.css?v=1"><link rel="stylesheet" href="/docs/raw/theme.css"><link rel="stylesheet" href="https://example.com/remote.css"><script src="https://example.com/injected.js"></script><script src="/docs/raw/untrusted.js"></script></head><body><aside class="file-sidebar">Full workspace tree</aside><main data-note-path="index.md">Original editor</main><script src="/assets/openknowledge/viewer.js?v=1"></script><script src="/assets/openknowledge/viewer-live-reload.js"></script></body></html>');
  });
  try {
    const prepared = await prepareNativeViewerResource(runtime.knowledge, 'window.OpenKnowledgeNativeRuntime={};');
    assert.deepEqual(prepared.bootstrap, { viewerURL: runtime.origin + '/docs/file/index.md', basePath: '/docs/' });
    assert.match(prepared.html, /Full workspace tree/);
    assert.match(prepared.html, /Original editor/);
    assert.match(prepared.html, /<style>\.file-sidebar/);
    assert.match(prepared.html, /body \{ color: green; \}/);
    assert.match(prepared.html, /https:\/\/example.com\/remote.css/);
    assert.match(prepared.html, /src="https:\/\/example.com\/injected.js"/);
    assert.match(prepared.html, /src="\/docs\/raw\/untrusted.js"/);
    assert.ok(!prepared.html.includes('src="/assets/openknowledge/'));
    assert.ok(!prepared.html.includes('viewer-live-reload'));
    assert.match(prepared.html, /function\(window,document,location,history,fetch,localStorage,sessionStorage\)/);
    assert.ok(prepared.html.indexOf('id="openknowledge-native-viewer-bootstrap"') < prepared.html.indexOf('window.theme'));
    assert.ok(prepared.html.indexOf('window.OpenKnowledgeNativeRuntime={};') < prepared.html.indexOf('window.theme'));
    assert.deepEqual(prepared.csp, { connectDomains: [], resourceDomains: [], frameDomains: [] });
    assert.equal(runtime.calls.some(call => call.route.includes('viewer-live-reload')), false);
    assert.equal(runtime.calls.some(call => call.route.includes('untrusted.js')), false, 'knowledge content is not promoted into trusted inline application code');
  } finally { await runtime.close(); }
});

test('native redirects and response size checks fail before exposing another origin or unbounded data', async () => {
  const runtime = await fixture((request, response) => {
    if (request.url === '/docs/external') { response.writeHead(302, { location: 'https://example.com/private' }).end(); return; }
    response.writeHead(200, { 'content-type': 'text/plain', 'content-length': String(33 * 1024 * 1024) }).end('tiny');
  });
  try {
    await assert.rejects(requestNativeViewer(runtime.knowledge, { route: '/docs/external' }), /cannot leave/);
    await assert.rejects(requestNativeViewer(runtime.knowledge, { route: '/docs/large' }), /exceeds 32 MiB/);
  } finally { await runtime.close(); }
});

test('SSR and API images are inert before browser parsing while their Markdown-facing URLs survive', async () => {
  const original = '<img src="../media/A%20B.png" alt="Note"><img src="data:image/png;base64,AA=="><source srcset="/raw/a.png 1x, /raw/b.png 2x"><img srcset="/raw/a.png 1x" src="/raw/a.png">';
  const prepared = prepareNativeHTMLImages(original);
  assert.match(prepared, /data-okn-native-image-src="\.\.\/media\/A%20B.png"/);
  assert.match(prepared, /src="data:image\/gif;base64/);
  assert.match(prepared, /src="data:image\/png;base64,AA=="/);
  assert.ok(!prepared.includes('srcset='));
  assert.equal(prepareNativeHTMLImages(prepared), prepared);
  const runtime = await fixture((request, response) => response.writeHead(200, { 'content-type': 'application/json' }).end(JSON.stringify({ body: original, markdown: '![Note](../media/A%20B.png)' })));
  try {
    const result = await requestNativeViewer(runtime.knowledge, { route: '/docs/api/file/note.md' });
    const data = JSON.parse(result.body);
    assert.equal(data.body, prepared);
    assert.equal(data.markdown, '![Note](../media/A%20B.png)');
  } finally { await runtime.close(); }
});
