const loopback = new Set(['127.0.0.1', 'localhost', '[::1]']);
const imageExtensions = /\.(?:png|jpe?g|gif|webp|avif|svg|bmp|ico|apng|tiff?)$/i;

export function nativeViewerURL(value) {
  const url = new URL(value);
  if (url.protocol !== 'http:' || !loopback.has(url.hostname) || url.username || url.password) throw new Error('The native viewer requires its local backend URL.');
  return url;
}

export function viewerRoute(value, currentURL) {
  const url = new URL(String(value), currentURL);
  if (url.origin !== new URL(currentURL).origin || url.username || url.password) throw new Error('Only the current viewer can be accessed through MCP.');
  const original = String(value).split(/[?#]/)[0];
  if (original.includes('\\') || /%2f|%5c/i.test(original) || original.split('/').some(part => ['.', '..'].includes(decodeURIComponent(part)))) {
    // Browser-relative document assets legitimately contain ../ and are
    // resolved separately before reaching this protected request boundary.
    throw new Error('Invalid viewer route.');
  }
  return url.pathname + url.search;
}

function memoryStorage() {
  const values = new Map();
  return { get length() { return values.size; }, key: index => [...values.keys()][index] ?? null,
    getItem: key => values.get(String(key)) ?? null, setItem: (key, value) => values.set(String(key), String(value)),
    removeItem: key => values.delete(String(key)), clear: () => values.clear() };
}

function availableStorage(windowObject, name) {
  try {
    const storage = windowObject[name];
    storage.getItem('openknowledge.native.storage-check');
    return storage;
  } catch { return memoryStorage(); }
}

export function responseFromViewer(payload, windowObject = globalThis) {
  if (!payload || !Number.isInteger(payload.status) || payload.status < 100 || payload.status > 599 || typeof payload.body !== 'string' || !['text', 'base64'].includes(payload.encoding)) throw new Error('Invalid native viewer response.');
  let body = payload.body;
  if (payload.encoding === 'base64') {
    const decoded = windowObject.atob(body);
    body = Uint8Array.from(decoded, character => character.charCodeAt(0));
  }
  return new windowObject.Response([101, 204, 205, 304].includes(payload.status) ? null : body, { status: payload.status, headers: payload.headers || {} });
}

// The original viewer scripts run with this lexical facade. Browser globals
// remain isolated by the host; no local HTTP request or actual navigation is
// needed to preserve the viewer's normal path and history semantics.
export function createNativeRuntime({ windowObject, bootstrap, callTool, onNavigate = () => {}, onError = () => {} }) {
  let current = nativeViewerURL(bootstrap.viewerURL);
  const nativeDocument = windowObject.document;
  const localStorage = availableStorage(windowObject, 'localStorage');
  const sessionStorage = availableStorage(windowObject, 'sessionStorage');
  let entries = [{ url: current.href, state: null }];
  let index = 0;
  let facade;
  const abort = signal => { if (signal?.aborted) throw signal.reason || new DOMException('The request was aborted.', 'AbortError'); };
  const route = value => viewerRoute(value, current.href);
  const resolve = value => new URL(String(value), current.href);
  const updateURL = (value, state, replace) => {
    const next = resolve(value == null ? current.href : value);
    route(next.href);
    current = next;
    if (replace) entries[index] = { url: current.href, state };
    else { entries = entries.slice(0, index + 1); entries.push({ url: current.href, state }); index++; }
    onNavigate(current.href);
  };
  const history = {
    get state() { return entries[index].state; }, get length() { return entries.length; }, scrollRestoration: 'manual',
    pushState: (state, unused, value) => updateURL(value, state, false),
    replaceState: (state, unused, value) => updateURL(value, state, true),
    go(delta = 0) {
      const next = index + Number(delta);
      if (!Number.isInteger(next) || next < 0 || next >= entries.length || next === index) return;
      index = next; current = new URL(entries[index].url); onNavigate(current.href);
      windowObject.dispatchEvent(new windowObject.PopStateEvent('popstate', { state: entries[index].state }));
    }, back() { history.go(-1); }, forward() { history.go(1); },
  };
  const location = {
    get href() { return current.href; }, set href(value) { location.assign(value); },
    get origin() { return current.origin; }, get protocol() { return current.protocol; }, get host() { return current.host; },
    get hostname() { return current.hostname; }, get port() { return current.port; }, get pathname() { return current.pathname; },
    get search() { return current.search; }, get hash() { return current.hash; },
    assign(value) { onNavigate(resolve(value).href, { requested: true }); },
    replace(value) { onNavigate(resolve(value).href, { requested: true, replace: true }); },
    reload() { onNavigate(current.href, { requested: true, reload: true }); }, toString: () => current.href,
  };
  async function fetchViewer(input, init = {}) {
    const Request = windowObject.Request;
    const request = input instanceof Request ? new Request(input, init) : new Request(resolve(input).href, init);
    abort(request.signal);
    const method = request.method.toUpperCase();
    if (!['GET', 'HEAD', 'POST'].includes(method)) throw new Error('Unsupported viewer request method.');
    const requestRoute = route(request.url);
    if (method === 'POST' && !new URL(request.url).pathname.endsWith('/api/document')) {
      return new windowObject.Response(JSON.stringify({ error: 'This action is available in the local browser viewer.', message: 'This action is available in the local browser viewer.' }), { status: 405, headers: { 'Content-Type': 'application/json' } });
    }
    const args = { route: requestRoute };
    if (method === 'POST') args.body = await request.text();
    const payload = await callTool(method === 'POST' ? 'openknowledge_viewer_save' : 'openknowledge_viewer_get', args, request.signal);
    abort(request.signal);
    if (method === 'HEAD') return new windowObject.Response(null, { status: payload.status, headers: payload.headers || {} });
    return responseFromViewer(payload, windowObject);
  }
  const boundWindowMethods = new Set(['addEventListener', 'removeEventListener', 'dispatchEvent', 'getSelection', 'matchMedia', 'getComputedStyle', 'requestAnimationFrame', 'cancelAnimationFrame', 'setTimeout', 'clearTimeout', 'setInterval', 'clearInterval', 'postMessage', 'scroll', 'scrollTo', 'scrollBy', 'alert', 'confirm', 'prompt', 'focus', 'blur', 'atob', 'btoa', 'queueMicrotask']);
  const replacements = { location, history, fetch: fetchViewer, localStorage, sessionStorage, EventSource: undefined };
  let memoryCookie = '';
  const document = nativeDocument && new Proxy(nativeDocument, {
    get(target, key) {
      if (key === 'cookie') { try { return target.cookie; } catch { return memoryCookie; } }
      if (key === 'defaultView') return facade;
      const value = Reflect.get(target, key, target);
      return typeof value === 'function' && key !== 'constructor' ? value.bind(target) : value;
    },
    set(target, key, value) {
      if (key === 'cookie') { try { target.cookie = value; } catch { memoryCookie = String(value).split(';')[0]; } return true; }
      return Reflect.set(target, key, value, target);
    },
  });
  replacements.document = document;
  facade = new Proxy(windowObject, {
    get(target, key) {
      if (Object.hasOwn(replacements, key)) return replacements[key];
      if (key === 'window' || key === 'self') return facade;
      if (key === 'open') return value => { onNavigate(resolve(value).href, { external: true }); return null; };
      const value = Reflect.get(target, key, target);
      return boundWindowMethods.has(key) && typeof value === 'function' ? value.bind(target) : value;
    }, set: (target, key, value) => Reflect.set(target, key, value, target),
  });
  const assetCache = new Map();
  async function assetURL(value, baseURL = current.href) {
    const url = new URL(String(value), baseURL);
    if (url.protocol === 'data:' || url.protocol === 'blob:') return url.href;
    if (url.origin !== current.origin) throw new Error('Remote assets are available in the browser viewer.');
    if (imageExtensions.test(url.pathname)) {
      const match = url.pathname.match(/^((?:\/kb\/[^/]+|\/[^/]+)?)\/file\/(.+)$/);
      if (match) url.pathname = match[1] + '/raw/' + match[2];
    }
    const key = route(url.href);
    if (!assetCache.has(key)) assetCache.set(key, (async () => {
      const payload = await callTool('openknowledge_viewer_get', { route: key });
      if (payload.status < 200 || payload.status >= 300) throw new Error('The viewer asset could not be read.');
      const mimeType = String(payload.headers?.['content-type'] || 'application/octet-stream').split(';')[0];
      if (!/^(image\/|audio\/|video\/|application\/pdf$|text\/css$)/i.test(mimeType)) throw new Error('The viewer asset has an unsupported display type.');
      const base64 = payload.encoding === 'base64' ? payload.body : windowObject.btoa(String.fromCharCode(...new TextEncoder().encode(payload.body)));
      return `data:${mimeType};base64,${base64}`;
    })().catch(error => { assetCache.delete(key); throw error; }));
    return assetCache.get(key);
  }
  return { window: facade, document, location, history, fetch: fetchViewer, localStorage, sessionStorage,
    resolve, route, assetURL, onError, get url() { return current.href; } };
}
