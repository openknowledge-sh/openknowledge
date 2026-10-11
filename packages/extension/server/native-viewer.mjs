const MAX_RESPONSE_BYTES = 32 * 1024 * 1024;
export const MAX_SAVE_BYTES = 6 * 1024 * 1024 + 64 * 1024;
const assetPrefix = '/assets/openknowledge/';
const loopback = new Set(['127.0.0.1', 'localhost', '[::1]']);
const imagePlaceholder = 'data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=';

export function prepareNativeHTMLImages(html) {
  return html.replace(/<(?:img|source)\b[^>]*>/gi, tag => {
    tag = tag.replace(/\s+srcset\s*=\s*(["'])(.*?)\1/gi, '');
    if (!/^<img\b/i.test(tag) || /\sdata-okn-native-image-src\s*=/i.test(tag)) return tag;
    return tag.replace(/\ssrc\s*=\s*(["'])(.*?)\1/i, (all, quote, value) => /^(?:data:|blob:)/i.test(value)
      ? all : ` src="${imagePlaceholder}" data-okn-native-image-src=${quote}${value}${quote}`);
  });
}

function prepareNativeJSONImages(value) {
  if (Array.isArray(value)) return value.map(prepareNativeJSONImages);
  if (!value || typeof value !== 'object') return value;
  return Object.fromEntries(Object.entries(value).map(([key, item]) => [key,
    typeof item === 'string' && ['body', 'frontmatter', 'claims'].includes(key) ? prepareNativeHTMLImages(item) : prepareNativeJSONImages(item),
  ]));
}

function viewerHome(value) {
  const url = new URL(value);
  if (url.protocol !== 'http:' || !loopback.has(url.hostname) || url.username || url.password || url.search || url.hash || !url.pathname.endsWith('/')) throw new Error('The native viewer requires a local viewer home.');
  return url;
}

export function viewerRequestURL(home, route, save = false) {
  const base = viewerHome(home);
  if (typeof route !== 'string' || !route.startsWith('/') || route.startsWith('//') || route.length > 8192 || /[\\#\u0000-\u001f\u007f]/.test(route)) throw new Error('Use an absolute viewer route without an origin or fragment.');
  const rawPath = route.split('?')[0];
  try {
    if (/%2f|%5c/i.test(rawPath) || rawPath.split('/').some(segment => ['.', '..'].includes(decodeURIComponent(segment)))) throw new Error();
  } catch { throw new Error('Invalid viewer route.'); }
  const target = new URL(route, base.origin);
  if (target.origin !== base.origin || !target.pathname.startsWith(base.pathname) && !(target.pathname.startsWith(assetPrefix) && !save)) throw new Error('The requested route is outside this viewer workspace.');
  const scopedPath = target.pathname.slice(base.pathname.length);
  if (save && scopedPath !== 'api/document' && !(base.pathname === '/' && /^(?:kb\/[^/]+|[^/]+)\/api\/document$/.test(scopedPath))) throw new Error('Only the native document save endpoint can write.');
  return target;
}

async function readBounded(response) {
  if (Number(response.headers.get('content-length')) > MAX_RESPONSE_BYTES) { await response.body?.cancel(); throw new Error('The viewer response exceeds 32 MiB.'); }
  const chunks = [];
  let size = 0;
  if (response.body) {
    const reader = response.body.getReader();
    try {
      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        size += value.byteLength;
        if (size > MAX_RESPONSE_BYTES) { await reader.cancel(); throw new Error('The viewer response exceeds 32 MiB.'); }
        chunks.push(Buffer.from(value));
      }
    } finally { reader.releaseLock(); }
  }
  return Buffer.concat(chunks, size);
}

async function fetchViewer(home, route, body) {
  const save = body !== undefined;
  if (save && (typeof body !== 'string' || Buffer.byteLength(body) > MAX_SAVE_BYTES)) throw new Error('The encoded save request exceeds the allowed size.');
  let target = viewerRequestURL(home, route, save);
  const origin = target.origin;
  for (let redirects = 0; redirects < 8; redirects++) {
    const response = await fetch(target, {
      method: save ? 'POST' : 'GET', redirect: 'manual', signal: AbortSignal.timeout(15000),
      ...(save ? { body, headers: { 'Content-Type': 'application/json', Origin: origin } } : {}),
    });
    if (!save && [301, 302, 303, 307, 308].includes(response.status) && response.headers.has('location')) {
      const location = new URL(response.headers.get('location'), target);
      await response.body?.cancel();
      if (location.origin !== origin || location.hash) throw new Error('A viewer redirect cannot leave its local origin.');
      target = viewerRequestURL(home, location.pathname + location.search);
      continue;
    }
    return { response, target, bytes: await readBounded(response) };
  }
  throw new Error('Too many viewer redirects.');
}

export async function requestNativeViewer(knowledge, { route, body }, save = false) {
  if (!knowledge.getViewerHome) throw new Error('The native viewer is unavailable.');
  if (save && typeof body !== 'string') throw new Error('Provide the document save JSON body.');
  if (!save && body !== undefined) throw new Error('Viewer reads do not accept a body.');
  const { viewerURL } = await knowledge.getViewerHome();
  // Rich editor Markdown image URLs resolve alongside /file/ pages. The raw
  // viewer handler retains its own root, extension and symlink safeguards.
  const image = !save && route.split('?')[0].match(/^((?:\/kb\/[^/]+|\/[^/]+)?)\/file\/(.+\.(?:png|jpe?g|gif|webp|avif|svg|bmp|ico|apng|tiff?))$/i);
  if (image) route = image[1] + '/raw/' + image[2] + route.slice(route.indexOf('?') === -1 ? route.length : route.indexOf('?'));
  const { response, target, bytes } = await fetchViewer(viewerURL, route, save ? body : undefined);
  const type = response.headers.get('content-type') || 'application/octet-stream';
  const text = /^(?:text\/[^;]+|application\/(?:json|javascript|x-javascript|xml))(?:[;\s]|$)/i.test(type);
  let output = bytes.toString(text ? 'utf8' : 'base64');
  if (!save && /\btext\/html\b/i.test(type)) output = prepareNativeHTMLImages(output);
  else if (!save && /\bapplication\/json\b/i.test(type)) {
    try { output = JSON.stringify(prepareNativeJSONImages(JSON.parse(output))); } catch {}
  }
  return { status: response.status, headers: { 'content-type': type, 'content-location': target.pathname + target.search }, body: output, encoding: text ? 'text' : 'base64' };
}

const safeJSON = value => JSON.stringify(value).replace(/[<>&\u2028\u2029]/g, character => '\\u' + character.charCodeAt(0).toString(16).padStart(4, '0'));
const safeScript = value => value.replace(/<\/script/gi, '<\\/script');

function wrappedScript(source) {
  return `(function(window,document,location,history,fetch,localStorage,sessionStorage){${source}\n})(window.OpenKnowledgeNativeRuntime.window,window.OpenKnowledgeNativeRuntime.document||document,window.OpenKnowledgeNativeRuntime.location,window.OpenKnowledgeNativeRuntime.history,window.OpenKnowledgeNativeRuntime.fetch,window.OpenKnowledgeNativeRuntime.window.localStorage,window.OpenKnowledgeNativeRuntime.window.sessionStorage);`;
}

// Load the same Go viewer document and bundled assets into the MCP resource.
// Browser requests and saves then use host-approved, app-only MCP tools.
export async function prepareNativeViewerResource(knowledge, adapterJS) {
  if (typeof adapterJS !== 'string' || !adapterJS.trim()) throw new Error('Build the native viewer adapter before opening this plugin.');
  const { viewerURL } = await knowledge.getViewerHome();
  const base = viewerHome(viewerURL);
  const { response, target, bytes } = await fetchViewer(viewerURL, base.pathname);
  if (!response.ok || !/\btext\/html\b/i.test(response.headers.get('content-type') || '')) throw new Error(`The native viewer returned ${response.status} instead of HTML.`);
  let html = prepareNativeHTMLImages(bytes.toString('utf8'));
  const cache = new Map();
  async function localAsset(value, trustedScript = false) {
    const url = new URL(value.replaceAll('&amp;', '&'), target);
    if (url.origin !== base.origin) return undefined;
    if (trustedScript && !url.pathname.startsWith(assetPrefix)) return undefined;
    const key = url.pathname + url.search;
    if (!cache.has(key)) cache.set(key, fetchViewer(viewerURL, key));
    const result = await cache.get(key);
    if (!result.response.ok) throw new Error(`The native viewer asset returned ${result.response.status}.`);
    return result;
  }
  const scripts = [...html.matchAll(/<script\b([^>]*?)\bsrc\s*=\s*(["'])(.*?)\2[^>]*>\s*<\/script\s*>/gi)];
  for (const script of scripts) {
    if (/viewer-live-reload(?:[.\/?]|$)/i.test(script[3])) { html = html.replace(script[0], ''); continue; }
    const asset = await localAsset(script[3], true);
    if (asset) html = html.replace(script[0], () => `<script>${safeScript(wrappedScript(asset.bytes.toString('utf8')))}</script>`);
  }
  const styles = [...html.matchAll(/<link\b(?=[^>]*\brel\s*=\s*["']stylesheet["'])(?=[^>]*\bhref\s*=\s*(["'])(.*?)\1)[^>]*>/gi)];
  for (const style of styles) {
    const asset = await localAsset(style[2]);
    if (asset) html = html.replace(style[0], () => `<style>${asset.bytes.toString('utf8').replace(/<\/style/gi, '<\\/style')}</style>`);
  }
  const bootstrap = { viewerURL: target.href, basePath: base.pathname };
  const early = `<script type="application/json" id="openknowledge-native-viewer-bootstrap">${safeJSON(bootstrap)}</script><script>${safeScript(adapterJS)}</script>`;
  html = html.replace(/<head\b[^>]*>/i, head => head + early);
  return { html, bootstrap, csp: { connectDomains: [], resourceDomains: [], frameDomains: [] } };
}
