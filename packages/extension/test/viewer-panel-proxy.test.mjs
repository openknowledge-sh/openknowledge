import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer, request } from 'node:http';
import { runInNewContext } from 'node:vm';
import { startViewerPanelProxy } from '../server/viewer-panel-proxy.mjs';

async function fixture(callback) {
  const calls = [];
  const viewer = createServer((request, response) => {
    calls.push({ url: request.url, method: request.method, origin: request.headers.origin, cookie: request.headers.cookie });
    response.setHeader('X-Frame-Options', 'DENY');
    if (request.url.startsWith('/api/file/')) {
      response.setHeader('Content-Type', 'application/json');
      response.end(JSON.stringify({ body: '<a href="/file/other.md">Other</a>', path: 'note.md' }));
    } else if (request.url === '/api/viewer-events') {
      response.writeHead(200, { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-store' });
      response.write('event: ready\ndata: {"revision":"initial"}\n\n');
    } else if (request.url === '/api/document') {
      response.setHeader('Content-Type', 'application/json');
      response.end(JSON.stringify({ saved: true, text: '![Viewer image](../media/pixel.png)' }));
    } else if (request.url === '/kb/alpha/raw/media/pixel.png') {
      response.setHeader('Content-Type', 'image/png');
      response.end(Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGPQSPb9DwADZwHYO30dDAAAAABJRU5ErkJggg==', 'base64'));
    } else if (request.url.startsWith('/assets/')) {
      response.setHeader('Content-Type', 'text/javascript');
      response.end('console.log("viewer");');
    } else if (request.url === '/redirect') {
      response.writeHead(302, { Location: '/file/other.md' }).end();
    } else {
      response.setHeader('Content-Type', 'text/html');
      response.setHeader('Content-Security-Policy', "default-src 'self'; script-src 'self'; frame-ancestors 'none'");
      response.end('<html><head><script src="/assets/viewer.js"></script></head><body><main data-link-prefix=""><a href="/file/note.md">Note</a><a href="https://example.com">External</a><img src="/raw/image.png"><section data-search-url="/api/search"></section></main><script type="application/json" data-editor-options>{"editors":[{"id":"local","name":"<Editor>","icon":"/api/editor-icon/test"},{"id":"external","icon":"https://example.com/icon.png"}]}</script></body></html>');
    }
  });
  await new Promise(resolve => viewer.listen(0, '127.0.0.1', resolve));
  const viewerURL = `http://127.0.0.1:${viewer.address().port}/`;
  const proxy = await startViewerPanelProxy({ viewerURL });
  try { await callback({ proxy, viewerURL, calls }); }
  finally { await proxy.close(); await new Promise(resolve => { viewer.close(resolve); viewer.closeAllConnections(); }); }
}

test('private viewer panel preserves the actual viewer scripts and safe headers while permitting only its HTML to be framed', async () => {
  await fixture(async ({ proxy, viewerURL }) => {
    const direct = await fetch(viewerURL + 'file/note.md');
    assert.equal(direct.headers.get('x-frame-options'), 'DENY');
    assert.ok(!(await direct.text()).includes('_embedded-viewer.css'), 'ordinary viewer layout remains unchanged');
    const panel = await fetch(proxy.urlFor(viewerURL + 'file/note.md?edit=1'));
    assert.equal(panel.headers.get('x-frame-options'), null);
    assert.equal(panel.headers.get('content-security-policy'), "default-src 'self'; script-src 'self'");
    assert.equal(panel.headers.get('referrer-policy'), 'no-referrer');
    const html = await panel.text();
    assert.ok(html.includes(`src="${proxy.prefix}/_viewer-bridge.js"`));
    assert.ok(html.includes(`href="${proxy.prefix}/_embedded-viewer.css"`));
    assert.ok(html.includes(`src="${proxy.prefix}/assets/viewer.js"`));
    assert.ok(html.includes(`data-link-prefix="${proxy.prefix}"`));
    assert.ok(html.includes(`href="${proxy.prefix}/file/note.md"`));
    assert.ok(html.includes('href="https://example.com"'));
    const editorJSON = JSON.parse(html.match(/<script type="application\/json" data-editor-options>(.*?)<\/script>/)[1]);
    assert.equal(editorJSON.editors[0].icon, proxy.prefix + '/api/editor-icon/test');
    assert.equal(editorJSON.editors[0].name, '<Editor>');
    assert.equal(editorJSON.editors[1].icon, 'https://example.com/icon.png');
    assert.ok(html.includes('\\u003cEditor>'), 'inline JSON serialization cannot introduce an executable HTML tag');
    const asset = await fetch(proxy.origin + proxy.prefix + '/assets/viewer.js');
    assert.equal(asset.headers.get('x-frame-options'), 'DENY');
    const json = await (await fetch(proxy.origin + proxy.prefix + '/api/file/note.md')).json();
    assert.ok(json.body.includes(`href="${proxy.prefix}/file/other.md"`));
    assert.equal(json.path, 'note.md');
  });
});

test('only GET and HEAD image assets use raw viewer routes while documents and Markdown sources remain unchanged', async () => {
  await fixture(async ({ proxy, viewerURL, calls }) => {
    const imageURL = proxy.urlFor(viewerURL + 'kb/alpha/file/media/pixel.png');
    const requestImage = method => new Promise((resolve, reject) => {
      request(imageURL, { method, headers: { 'Sec-Fetch-Dest': 'image' } }, response => {
        const chunks = [];
        response.on('data', chunk => chunks.push(chunk));
        response.once('end', () => resolve({ status: response.statusCode, contentType: response.headers['content-type'], body: Buffer.concat(chunks) }));
      }).once('error', reject).end();
    });
    const image = await requestImage('GET');
    assert.equal(image.status, 200);
    assert.equal(image.contentType, 'image/png');
    assert.equal(image.body.subarray(0, 8).toString('hex'), '89504e470d0a1a0a');
    assert.equal(calls.at(-1).url, '/kb/alpha/raw/media/pixel.png');
    const head = await requestImage('HEAD');
    assert.equal(head.contentType, 'image/png');
    assert.equal(head.body.length, 0);
    assert.equal(calls.at(-1).url, '/kb/alpha/raw/media/pixel.png');
    const document = await fetch(imageURL);
    assert.equal(document.headers.get('content-type'), 'text/html');
    assert.equal(calls.at(-1).url, '/kb/alpha/file/media/pixel.png', 'opening an asset retains the native preview route');
    await fetch(imageURL, { method: 'POST', headers: { 'Sec-Fetch-Dest': 'image', Origin: proxy.origin } });
    assert.equal(calls.at(-1).url, '/kb/alpha/file/media/pixel.png', 'mutations never use the raw image route');
    await fetch(proxy.urlFor(viewerURL + 'kb/alpha/file/note.md'), { headers: { 'Sec-Fetch-Dest': 'image' } });
    assert.equal(calls.at(-1).url, '/kb/alpha/file/note.md', 'Markdown cannot be read through the raw route');
    const source = await (await fetch(proxy.urlFor(viewerURL + 'api/document'))).json();
    assert.equal(source.text, '![Viewer image](../media/pixel.png)');
  });
});

test('embedded layout stays private and the proxy forwards live reload events without buffering the connection', async () => {
  await fixture(async ({ proxy }) => {
    assert.equal((await fetch(proxy.origin + '/_embedded-viewer.css')).status, 404);
    const stylesheet = await fetch(proxy.origin + proxy.prefix + '/_embedded-viewer.css');
    assert.equal(stylesheet.headers.get('content-type'), 'text/css; charset=utf-8');
    const css = await stylesheet.text();
    assert.match(css, /\.note-panel\.document.*flex-basis: min\(.*max-width: 100%/);
    assert.match(css, /\.file-sidebar.*position: absolute/);
    assert.match(css, /@media \(max-width: 680px\)/);
    assert.ok(!css.includes('.file-sidebar-resize'), 'the full viewer retains its native desktop resize controls');
    const abort = new AbortController();
    try {
      const response = await fetch(proxy.origin + proxy.prefix + '/api/viewer-events', { headers: { Origin: proxy.origin }, signal: abort.signal });
      assert.equal(response.status, 200);
      assert.equal(response.headers.get('content-type'), 'text/event-stream');
      const stream = response.body.getReader();
      const first = await stream.read();
      assert.match(new TextDecoder().decode(first.value), /event: ready\ndata: \{"revision":"initial"\}/);
      await stream.cancel();
    } finally { abort.abort(); }
  });
});

test('whole-viewer home handshakes retain native sidebar state and do not reload homes or canonical redirects', async () => {
  await fixture(async ({ proxy, viewerURL }) => {
    const script = await (await fetch(proxy.origin + proxy.prefix + '/_viewer-bridge.js')).text();
    const homeURL = proxy.urlFor(viewerURL);
    const handlers = new Map();
    const documentHandlers = new Map();
    const messages = [];
    const redirects = [];
    let sidebarClicks = 0;
    let mutation;
    const sidebarToggle = { click() { sidebarClicks++; }, addEventListener() {} };
    let panel = null;
    const parent = { postMessage: data => messages.push(data) };
    const document = {
      documentElement: { dataset: { viewerView: 'notes' } },
      body: { dataset: {}, classList: { contains: name => name === 'is-sidebar-open' } },
      addEventListener: (type, listener) => documentHandlers.set(type, listener),
      querySelector: selector => selector === '[data-sidebar-toggle]' ? sidebarToggle : panel,
      querySelectorAll: () => [],
    };
    const window = {
      location: { href: homeURL, pathname: new URL(homeURL).pathname, origin: proxy.origin, assign: url => redirects.push(url) },
      parent, fetch: async () => {}, history: { pushState() {}, replaceState() {} }, getSelection: () => null,
      addEventListener: (type, listener) => handlers.set(type, listener),
    };
    runInNewContext(script, { window, document, URL, Request, queueMicrotask, MutationObserver: class { constructor(listener) { mutation = listener; } observe() {} } });
    documentHandlers.get('DOMContentLoaded')();
    assert.equal(sidebarClicks, 0, 'the full viewer owns its sidebar state');
    const send = data => handlers.get('message')({ source: parent, origin: 'http://127.0.0.1:4318', data: { protocol: 'openknowledge-viewer', version: 1, ...data } });
    send({ type: 'initialize', channel: 'home', url: homeURL });
    assert.equal(messages.find(message => message.type === 'ready').channel, 'home');
    assert.equal(messages.find(message => message.type === 'page').page, null);
    send({ type: 'navigate', channel: 'same-home', url: homeURL });
    assert.equal(redirects.length, 0);
    const canonical = proxy.urlFor(viewerURL + 'kb/only/file/index.md');
    window.location.href = canonical;
    window.location.pathname = new URL(canonical).pathname;
    panel = { dataset: { notePath: 'index.md', knowledgeBase: 'only', noteTitle: 'Home' } };
    send({ type: 'initialize', channel: 'redirected-home', url: homeURL });
    assert.equal(redirects.length, 0, 'a server redirect cannot cause initialize/reload loops');
    assert.equal(messages.find(message => message.type === 'page' && message.channel === 'redirected-home').page.knowledgeBase, 'only');
    document.documentElement.dataset.viewerView = 'graph';
    mutation();
    assert.equal(messages.filter(message => message.type === 'page').at(-1).page, null, 'a graph or claims view cannot expose a hidden saved page as the current document');
  });
});

test('panel token, Host and same-origin mutation requirements reject untrusted requests before forwarding', async () => {
  await fixture(async ({ proxy, viewerURL, calls }) => {
    assert.equal((await fetch(proxy.origin + '/file/note.md')).status, 404);
    const spoofedHostStatus = await new Promise((resolve, reject) => {
      request(proxy.urlFor(viewerURL + 'file/note.md'), { headers: { Host: 'attacker.example' } }, response => {
        response.resume();
        response.once('end', () => resolve(response.statusCode));
      }).once('error', reject).end();
    });
    assert.equal(spoofedHostStatus, 404);
    const mutation = proxy.origin + proxy.prefix + '/api/document';
    for (const origin of [undefined, 'null', 'https://example.com']) {
      const response = await fetch(mutation, { method: 'POST', headers: { 'Content-Type': 'application/json', ...(origin ? { Origin: origin } : {}) }, body: '{}' });
      assert.equal(response.status, 403);
    }
    assert.equal(calls.length, 0);
    const saved = await fetch(mutation, { method: 'POST', headers: { 'Content-Type': 'application/json', Origin: proxy.origin, Cookie: 'not-forwarded=1' }, body: '{}' });
    assert.equal(saved.status, 200);
    assert.equal(calls.at(-1).origin, new URL(viewerURL).origin);
    assert.equal(calls.at(-1).cookie, undefined);
    assert.throws(() => proxy.urlFor('http://127.0.0.1:9/file/note.md'), /must belong/);
  });
});

test('viewer redirects and isolated MCP test resources retain the protected path', async () => {
  await fixture(async ({ proxy, viewerURL }) => {
    const redirected = await fetch(proxy.urlFor(viewerURL + 'redirect'), { redirect: 'manual' });
    assert.equal(redirected.headers.get('location'), proxy.prefix + '/file/other.md');
    const appURL = proxy.getAppURL('<h1>Built MCP App</h1>');
    assert.equal(new URL(appURL).origin, proxy.origin);
    const appResponse = await fetch(appURL);
    assert.match(appResponse.headers.get('content-security-policy'), /connect-src 'none'/);
    assert.ok(appResponse.headers.get('content-security-policy').includes('frame-src ' + proxy.origin));
    assert.equal(await appResponse.text(), '<h1>Built MCP App</h1>');
    const bridge = await (await fetch(proxy.origin + proxy.prefix + '/_viewer-bridge.js')).text();
    assert.match(bridge, /openknowledge-viewer/);
    assert.match(bridge, /context-request/);
    assert.doesNotThrow(() => new Function(bridge));
  });
});

test('viewer SPA navigation rejects spoofed parent messages and waits for the requested active page before announcing readiness', async () => {
  await fixture(async ({ proxy, viewerURL }) => {
    const script = await (await fetch(proxy.origin + proxy.prefix + '/_viewer-bridge.js')).text();
    const handlers = new Map();
    const documentHandlers = new Map();
    const messages = [];
    const redirects = [];
    const initialURL = proxy.urlFor(viewerURL + 'file/note.md');
    const nextURL = proxy.urlFor(viewerURL + 'file/other.md');
    const parent = { postMessage: data => messages.push(data) };
    const panel = { dataset: { notePath: 'note.md', knowledgeBase: 'test', noteTitle: 'Note' } };
    let clicks = 0;
    let mutation;
    const anchor = { href: nextURL, dataset: { knowledgeBase: 'test' }, click: () => { clicks++; } };
    const document = {
      body: { dataset: { activeKnowledgeBase: 'test' } },
      addEventListener: (type, listener) => documentHandlers.set(type, listener),
      querySelector: () => panel,
      querySelectorAll: () => [anchor],
    };
    const window = {
      location: { href: initialURL, pathname: new URL(initialURL).pathname, origin: proxy.origin, assign: url => redirects.push(url) },
      parent, fetch: async () => {}, history: { pushState() {}, replaceState() {} }, getSelection: () => null,
      addEventListener: (type, listener) => handlers.set(type, listener),
    };
    runInNewContext(script, { window, document, URL, Request, queueMicrotask, MutationObserver: class { constructor(listener) { mutation = listener; } observe() {} } });
    documentHandlers.get('DOMContentLoaded')();
    const origin = 'http://127.0.0.1:4318';
    const send = (data, sender = parent, senderOrigin = origin) => handlers.get('message')({ source: sender, origin: senderOrigin, data: { protocol: 'openknowledge-viewer', version: 1, ...data } });
    send({ type: 'initialize', channel: 'initial', url: initialURL });
    assert.equal(messages.find(message => message.type === 'ready').channel, 'initial');
    messages.length = 0;
    const navigation = { type: 'navigate', channel: 'new', url: nextURL };
    send(navigation, {});
    send(navigation, parent, 'https://example.com');
    send({ ...navigation, url: 'https://example.com/file/other.md' });
    send({ ...navigation, url: proxy.origin + '/wrong-token/file/other.md' });
    assert.equal(clicks, 0);
    assert.equal(messages.length, 0);
    send(navigation);
    assert.equal(clicks, 1);
    assert.equal(messages.some(message => message.type === 'ready'), false);
    window.location.pathname = new URL(nextURL).pathname;
    mutation();
    assert.equal(messages.some(message => message.type === 'ready'), false, 'matching URL alone cannot expose the previous active page');
    panel.dataset.notePath = 'other.md';
    mutation();
    assert.equal(messages.filter(message => message.type === 'ready').length, 1);
    const activePage = messages.find(message => message.type === 'page' && message.page);
    assert.equal(activePage.page.path, 'other.md');
    assert.equal(activePage.channel, 'new');
    const missingURL = proxy.urlFor(viewerURL + 'file/missing.md');
    send({ type: 'navigate', channel: 'missing', url: missingURL });
    assert.deepEqual(redirects, [missingURL]);
  });
});
