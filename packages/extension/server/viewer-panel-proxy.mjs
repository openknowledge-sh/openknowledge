import { createServer, request as httpRequest } from 'node:http';
import { randomBytes } from 'node:crypto';

const loopback = new Set(['127.0.0.1', 'localhost', '[::1]']);
const hopHeaders = new Set(['connection', 'keep-alive', 'proxy-authenticate', 'proxy-authorization', 'te', 'trailer', 'transfer-encoding', 'upgrade']);

// Preserve the complete viewer and its desktop layout. These private responsive
// adjustments fit documents to a narrow host and keep its sidebar toggle usable.
const embeddedViewerCSS = `
@media (max-width: 1100px) {
  body.viewer-document { grid-template-rows: auto minmax(0, 1fr); }
  body.viewer-document > header { height: auto; min-height: 48px; padding: 8px 12px; flex-wrap: wrap; gap: 8px; justify-content: space-between; }
  body.viewer-document > header .header-left { position: static; transform: none; max-width: none !important; flex: 0 1 auto; }
  body.viewer-document > header .brand { max-width: 160px; }
  body.viewer-document > header .search.header-search { flex: 1 1 180px; width: auto; min-width: 0; margin: 0; }
  body.viewer-document > header .graph-view-toggle { position: static; transform: none; flex: 0 0 auto; }
  body.viewer-document .note-stack { box-sizing: border-box; flex-basis: 100%; min-width: 100%; width: 100%; padding: 8px 12px 24px !important; }
  body.viewer-document .note-panel.document { flex-basis: min(var(--note-panel-width, var(--ok-note-panel-default-width, 780px)), 100%); min-width: min(240px, 100%); max-width: 100%; padding: 0 16px 24px; }
  body.viewer-document .note-chrome { flex-wrap: wrap; gap: 6px; margin: 0 -16px 12px; padding: 8px 12px; }
  body.viewer-document .note-actions { flex: 0 1 auto; min-width: 0; max-width: 100%; flex-wrap: wrap; }
  body.viewer-document .document-editor { min-width: 0; padding: 8px 0 20px; }
  body.viewer-document .document-editor-toolbar, body.viewer-document .document-rich-format { min-width: 0; flex-wrap: wrap; }
  body.viewer-document .document-editor-status { flex: 1 1 100%; }
  body.viewer-document .document-rich-content, body.viewer-document .note-body { min-width: 0; max-width: 100%; overflow-wrap: anywhere; }
}
@media (max-width: 680px) {
  body.viewer-document { --ok-sidebar-layout-width: 0px !important; }
  body.viewer-document > .file-sidebar { position: absolute; grid-column: 2; grid-row: 2; inset: 0 auto 0 0; width: min(320px, 90vw); height: 100%; z-index: 20; }
}
@media (max-width: 460px) {
  body.viewer-document > header .brand { max-width: 90px; }
  body.viewer-document > header .search.header-search { flex-basis: 130px; }
  body.viewer-document .note-path { flex-basis: 100%; }
}
`;

function viewerBase(value) {
  const url = new URL(value);
  if (url.protocol !== 'http:' || !loopback.has(url.hostname) || url.username || url.password || url.search || url.hash) {
    throw new Error('The viewer panel requires a loopback HTTP viewer URL.');
  }
  if (!url.pathname.endsWith('/')) url.pathname += '/';
  return url;
}

// The normal viewer keeps its framing protection. Only an unguessable,
// extension-owned route can serve this embedded copy of the same application.
export async function startViewerPanelProxy({ viewerURL }) {
  const base = viewerBase(viewerURL);
  const prefix = '/' + randomBytes(32).toString('hex');
  const basePath = base.pathname.replace(/\/$/, '');
  let origin;
  let appHTML;
  const upstreamRequests = new Set();

  function prefixURL(value) {
    if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//') || value === prefix || value.startsWith(prefix + '/')) return value;
    const route = basePath && (value === basePath || value.startsWith(basePath + '/')) ? value.slice(basePath.length) : value;
    return prefix + (route || '/');
  }

  function rewriteHTML(html) {
    const rewritten = html.replace(/\b(href|src|action|poster|data-(?:link-prefix|search-url|claims-url|graph-url|url))\s*=\s*(["'])(.*?)\2/gi, (all, name, quote, value) => {
      const result = name.toLowerCase() === 'data-link-prefix' && value === '' ? prefix : prefixURL(value);
      return `${name}=${quote}${result}${quote}`;
    });
    // Viewer editor icons and graph metadata also arrive as inert JSON in the
    // initial HTML. Rewrite their URL fields before the viewer consumes them.
    const withJSON = rewritten.replace(/(<script\b(?=[^>]*\btype\s*=\s*["']application\/json["'])[^>]*>)([\s\S]*?)(<\/script\s*>)/gi, (all, opening, body, closing) => {
      try { return opening + JSON.stringify(rewriteJSON(JSON.parse(body))).replaceAll('<', '\\u003c') + closing; }
      catch { return all; }
    });
    const bridge = `<script src="${prefix}/_viewer-bridge.js"></script>`;
    return withJSON.replace(/<head\b[^>]*>/i, head => head + bridge)
      .replace(/<\/head\s*>/i, `<link rel="stylesheet" href="${prefix}/_embedded-viewer.css"></head>`);
  }

  function rewriteJSON(data) {
    if (Array.isArray(data)) return data.map(rewriteJSON);
    if (!data || typeof data !== 'object') return data;
    return Object.fromEntries(Object.entries(data).map(([key, value]) => [key,
      typeof value === 'string' && ['body', 'frontmatter', 'claims'].includes(key) ? rewriteHTML(value)
        : typeof value === 'string' && (/url$/i.test(key) || ['href', 'icon'].includes(key)) ? prefixURL(value)
          : rewriteJSON(value),
    ]));
  }

  const server = createServer((incoming, outgoing) => {
    const expectedHost = new URL(origin).host;
    if (incoming.headers.host !== expectedHost || !incoming.url?.startsWith(prefix + '/')) {
      outgoing.writeHead(404).end('Viewer panel unavailable.');
      return;
    }
    const read = incoming.method === 'GET' || incoming.method === 'HEAD';
    if (incoming.headers.origin ? incoming.headers.origin !== origin : !read) {
      outgoing.writeHead(403).end('A same-origin viewer request is required.');
      return;
    }
    let route;
    try {
      route = new URL(incoming.url.slice(prefix.length), base.origin);
      // Reject ambiguous traversal before it can resolve outside the viewer.
      const rawPath = incoming.url.split('?')[0];
      if (rawPath.includes('\\') || /%2f|%5c/i.test(rawPath) || rawPath.split('/').some(segment => ['.', '..'].includes(decodeURIComponent(segment)))) throw new Error('Invalid route.');
      if (route.origin !== base.origin) throw new Error('Invalid route.');
    } catch {
      outgoing.writeHead(400).end('Invalid viewer route.');
      return;
    }
    if (route.pathname === '/_viewer-bridge.js') {
      if (!read) { outgoing.writeHead(405).end(); return; }
      outgoing.writeHead(200, { 'Content-Type': 'text/javascript; charset=utf-8', 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' });
      outgoing.end(incoming.method === 'HEAD' ? undefined : `(${viewerBridge.toString()})(${JSON.stringify(prefix)});`);
      return;
    }
    if (route.pathname === '/_embedded-viewer.css') {
      if (!read) { outgoing.writeHead(405).end(); return; }
      outgoing.writeHead(200, { 'Content-Type': 'text/css; charset=utf-8', 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' });
      outgoing.end(incoming.method === 'HEAD' ? undefined : embeddedViewerCSS);
      return;
    }
    if (route.pathname === '/_mcp-app') {
      if (!read || !appHTML) { outgoing.writeHead(404).end(); return; }
      outgoing.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store', 'Referrer-Policy': 'no-referrer', 'X-Content-Type-Options': 'nosniff',
        'Content-Security-Policy': `default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data:; frame-src ${origin}; connect-src 'none'; base-uri 'none'; object-src 'none'` });
      outgoing.end(incoming.method === 'HEAD' ? undefined : appHTML);
      return;
    }
    const target = new URL(base);
    // The rich editor preserves Markdown image URLs, so relative images resolve
    // beside the visible /file/ document route. Read those image assets through
    // the viewer's raw handler, which keeps its existing filesystem safeguards.
    // Document navigation, Markdown sources, and mutations keep their routes.
    const imageRoute = read && incoming.headers['sec-fetch-dest'] === 'image'
      ? route.pathname.match(/^((?:\/kb\/[^/]+|\/[^/]+)?)\/file\/(.+\.(?:png|jpe?g|gif|webp|avif|svg|bmp|ico|apng|tiff?))$/i) : null;
    target.pathname = basePath + (imageRoute ? imageRoute[1] + '/raw/' + imageRoute[2] : route.pathname);
    target.search = route.search;
    target.searchParams.delete('_okn_frame');
    const headers = Object.fromEntries(Object.entries(incoming.headers).filter(([key]) => !hopHeaders.has(key) && !['host', 'cookie', 'authorization', 'accept-encoding', 'origin'].includes(key)));
    headers.host = base.host;
    if (incoming.headers.origin) headers.origin = base.origin;
    const upstream = httpRequest(target, { method: incoming.method, headers }, response => {
      const contentType = String(response.headers['content-type'] || '');
      const html = /\btext\/html\b/i.test(contentType);
      const json = /\bapplication\/json\b/i.test(contentType);
      const outputHeaders = Object.fromEntries(Object.entries(response.headers).filter(([key]) => !hopHeaders.has(key) && !['set-cookie', 'content-length', 'content-encoding', 'etag'].includes(key) && !(html && key === 'x-frame-options')));
      outputHeaders['cache-control'] = 'no-store';
      outputHeaders['referrer-policy'] = 'no-referrer';
      if (html && outputHeaders['content-security-policy']) {
        outputHeaders['content-security-policy'] = String(outputHeaders['content-security-policy']).split(';').filter(directive => !/^\s*frame-ancestors\b/i.test(directive)).join(';');
      }
      if (outputHeaders.location) {
        const redirected = new URL(String(outputHeaders.location), target);
        if (redirected.origin === base.origin) outputHeaders.location = prefixURL(redirected.pathname) + redirected.search + redirected.hash;
      }
      if (incoming.method === 'HEAD' || !html && !json) {
        outgoing.writeHead(response.statusCode, outputHeaders);
        response.pipe(outgoing);
        return;
      }
      const chunks = [];
      let size = 0;
      response.on('data', chunk => {
        size += chunk.length;
        if (size > 32 * 1024 * 1024) { response.destroy(); outgoing.writeHead(502).end('Viewer response exceeds the panel limit.'); return; }
        chunks.push(chunk);
      });
      response.on('end', () => {
        if (outgoing.destroyed || outgoing.writableEnded) return;
        let body = Buffer.concat(chunks).toString('utf8');
        try { body = html ? rewriteHTML(body) : JSON.stringify(rewriteJSON(JSON.parse(body))); }
        catch { outgoing.writeHead(502).end('Invalid viewer response.'); return; }
        outgoing.writeHead(response.statusCode, outputHeaders).end(body);
      });
      response.on('error', () => { if (!outgoing.writableEnded) outgoing.destroy(); });
    });
    upstreamRequests.add(upstream);
    upstream.on('close', () => upstreamRequests.delete(upstream));
    upstream.on('error', () => { if (!outgoing.headersSent) outgoing.writeHead(502).end('The local viewer is unavailable.'); else outgoing.destroy(); });
    incoming.on('aborted', () => upstream.destroy());
    outgoing.on('close', () => upstream.destroy());
    incoming.pipe(upstream);
  });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
  origin = `http://127.0.0.1:${server.address().port}`;
  return {
    origin, prefix,
    urlFor(value) {
      const source = new URL(value);
      if (source.origin !== base.origin || source.username || source.password || !source.pathname.startsWith(base.pathname)) throw new Error('The panel URL must belong to its local viewer.');
      return origin + prefixURL(source.pathname) + source.search + source.hash;
    },
    getAppURL(html) {
      if (typeof html !== 'string') throw new Error('The test app must be HTML.');
      appHTML = html;
      return origin + prefix + '/_mcp-app';
    },
    async close() {
      for (const request of upstreamRequests) request.destroy();
      await new Promise(resolve => { server.close(resolve); server.closeAllConnections(); });
    },
  };
}

// Runs before the viewer's own scripts. Route translation keeps fetch, SSE,
// history and rendered links on the protected proxy without changing the viewer.
function viewerBridge(prefix) {
  function routed(value) {
    const url = new URL(String(value), window.location.href);
    if (url.origin === window.location.origin && !url.pathname.startsWith(prefix + '/')) url.pathname = prefix + url.pathname;
    return url.href;
  }
  const fetchOriginal = window.fetch.bind(window);
  window.fetch = (input, init) => fetchOriginal(input instanceof Request ? new Request(routed(input.url), input) : routed(input), init);
  if (window.EventSource) {
    const Original = window.EventSource;
    window.EventSource = class extends Original { constructor(url, options) { super(routed(url), options); } };
  }
  for (const method of ['pushState', 'replaceState']) {
    const original = window.history[method].bind(window.history);
    window.history[method] = (state, unused, url) => original(state, unused, url == null ? url : routed(url));
  }
  let parentOrigin;
  let channel;
  let pageKey;
  let contextKey;
  let navigation;
  const post = data => window.parent.postMessage({ protocol: 'openknowledge-viewer', version: 1, channel, ...data }, parentOrigin && parentOrigin !== 'null' ? parentOrigin : '*');
  const documentsVisible = () => !document.documentElement?.dataset.viewerView || document.documentElement.dataset.viewerView === 'notes';
  const activeDocument = () => documentsVisible() ? document.querySelector('[data-note-path].is-active-panel') : null;
  function context() {
    if (!documentsVisible()) return null;
    const selection = window.getSelection();
    const element = selection?.anchorNode?.nodeType === 1 ? selection.anchorNode : selection?.anchorNode?.parentElement;
    const panel = element?.closest('[data-note-path]');
    const scope = panel?.querySelector('.document-rich-content') || panel?.querySelector('.note-body');
    const inside = panel?.isConnected && scope && scope.contains(selection.anchorNode) && scope.contains(selection.focusNode);
    const selected = inside ? selection.toString().trim() : '';
    const text = selected.slice(0, 24000);
    const status = panel?.querySelector('.document-editor-status')?.textContent || '';
    return text ? { text, path: panel.dataset.notePath, knowledgeBase: panel.dataset.knowledgeBase || document.body.dataset.activeKnowledgeBase || '', title: panel.dataset.noteTitle || panel.dataset.notePath, selection: true, truncated: selected.length > 24000, dirty: /not saved|unsaved|draft|saving|changed/i.test(status), contentKind: 'rendered-selection' } : null;
  }
  const sendContext = () => {
    if (!channel || navigation) return;
    const snapshot = context();
    const key = JSON.stringify(snapshot);
    if (key === contextKey) return;
    contextKey = key;
    post({ type: 'context', context: snapshot });
  };
  function sendPage(force = false) {
    if (!channel) return;
    if (navigation && !finishNavigation()) return;
    const panel = activeDocument();
    const page = panel ? { path: panel.dataset.notePath, knowledgeBase: panel.dataset.knowledgeBase || document.body.dataset.activeKnowledgeBase || '', title: panel.dataset.noteTitle || panel.dataset.notePath } : null;
    const key = JSON.stringify(page);
    if (!force && key === pageKey) return;
    pageKey = key;
    contextKey = undefined;
    post({ type: 'page', page });
    post({ type: 'context', context: null });
  }
  function announce() { post({ type: 'ready' }); sendPage(true); sendContext(); }
  function finishNavigation() {
    if (!navigation) return false;
    const panel = activeDocument();
    const knowledgeBase = panel?.dataset.knowledgeBase || document.body.dataset.activeKnowledgeBase || '';
    if (window.location.pathname !== navigation.url.pathname || panel?.dataset.notePath !== navigation.path || navigation.knowledgeBase !== undefined && knowledgeBase !== navigation.knowledgeBase) return false;
    navigation = undefined;
    announce();
    return true;
  }
  function navigate(value) {
    let url;
    try {
      url = new URL(value);
      if (url.origin !== window.location.origin || url.username || url.password || !url.pathname.startsWith(prefix + '/')) return;
    } catch { return; }
    const marker = url.pathname.indexOf('/file/', prefix.length);
    if (marker < 0) {
      // Root, workspace, graph and claims URLs are valid viewer destinations.
      // A handshake on the current home must never reload that same home.
      if (url.href === window.location.href) { navigation = undefined; announce(); }
      else window.location.assign(url.href);
      return;
    }
    let path;
    try { path = decodeURIComponent(url.pathname.slice(marker + 6)); } catch { return; }
    const anchor = Array.from(document.querySelectorAll('a[data-tree-path]')).find(link => {
      try { return new URL(link.href, window.location.href).pathname === url.pathname; } catch { return false; }
    });
    navigation = { url, path, knowledgeBase: anchor?.dataset.knowledgeBase };
    pageKey = undefined;
    contextKey = undefined;
    post({ type: 'page', page: null });
    post({ type: 'context', context: null });
    if (finishNavigation()) return;
    if (!anchor) { window.location.assign(url.href); return; }
    // The existing viewer's own navigation keeps draft sessions and Undo alive.
    // Start from the first panel so its stack URL becomes the requested page.
    // Invert a "beside" preference for this explicit catalog navigation only.
    document.querySelector('[data-note-workspace] .note-panel')?.focus?.();
    if (document.documentElement?.dataset.viewerNavigationMode === 'beside') {
      anchor.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, shiftKey: true }));
    } else anchor.click();
    queueMicrotask(finishNavigation);
  }
  window.addEventListener('message', event => {
    const data = event.data;
    if (event.source !== window.parent || data?.protocol !== 'openknowledge-viewer' || data.version !== 1 || typeof data.channel !== 'string' || data.channel.length > 256) return;
    if (parentOrigin !== undefined && event.origin !== parentOrigin) return;
    if (data.type === 'initialize') {
      parentOrigin = event.origin;
      channel = data.channel;
      // Initialization reports the document actually loaded, including a
      // canonical redirect. Explicit repeated opens can request navigation.
      if (data.url && data.navigate === true) navigate(data.url);
      else { navigation = undefined; announce(); }
    } else if (event.origin === parentOrigin && data.type === 'navigate') {
      // Validate the destination before switching the channel or page state.
      try {
        const target = new URL(data.url);
        if (target.origin !== window.location.origin || target.username || target.password || !target.pathname.startsWith(prefix + '/')) return;
      } catch { return; }
      channel = data.channel;
      navigate(data.url);
    } else if (data.channel === channel && event.origin === parentOrigin && data.type === 'context-request') sendContext();
  });
  document.addEventListener('selectionchange', sendContext);
  document.addEventListener('input', () => queueMicrotask(sendContext));
  window.addEventListener('popstate', () => { if (channel) post({ type: 'context', context: null }); queueMicrotask(() => sendPage()); });
  document.addEventListener('DOMContentLoaded', () => {
    const observer = new MutationObserver(() => { sendPage(); sendContext(); });
    observer.observe(document.body, { subtree: true, childList: true, attributes: true, attributeFilter: ['class', 'data-note-path', 'data-knowledge-base', 'data-note-title'] });
    if (document.documentElement) observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-viewer-view'] });
  });
  window.addEventListener('load', () => post({ type: 'loaded' }));
}
