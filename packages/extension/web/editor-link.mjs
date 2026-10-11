export function normalizeEditorURL(value) {
  let url;
  try { url = new URL(value); } catch { throw new Error('The editor must be served by the local Open Knowledge viewer.'); }
  if (url.protocol !== 'http:' || !['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname) || url.username || url.password) {
    throw new Error('The editor must be served by the local Open Knowledge viewer.');
  }
  return url;
}

export function frameAllowedByHost(value, host, declaredFrameDomains) {
  const url = normalizeEditorURL(value);
  if (!host) return true;
  const validDomains = domains => Array.isArray(domains) && domains.every(domain => typeof domain === 'string');
  const declared = validDomains(declaredFrameDomains) && declaredFrameDomains.includes(url.origin);
  if (declaredFrameDomains !== undefined && !declared) return false;
  const record = value => value !== null && typeof value === 'object' && !Array.isArray(value);
  if (typeof host.getHostCapabilities !== 'function') return false;
  const capabilities = host.getHostCapabilities();
  if (capabilities !== undefined && !record(capabilities)) return false;
  const sandbox = capabilities?.sandbox;
  if (sandbox !== undefined && !record(sandbox)) return false;
  const csp = sandbox?.csp;
  if (csp !== undefined && !record(csp)) return false;
  const reported = csp?.frameDomains;
  // Hosts can omit this optional capability report while still enforcing the
  // resource's CSP. Probe only its declared origin; browser CSP and the trusted
  // ready handshake decide whether the viewer actually works. An explicit
  // denial or malformed report must never be replaced by the declaration.
  return reported === undefined ? declared : validDomains(reported) && reported.includes(url.origin);
}

// Mount the real viewer, including its existing draft, Undo and Save controls.
// A successful iframe load alone does not prove that the host sandbox supports
// editing. The trusted proxy bridge must acknowledge a source/origin handshake.
export function createViewerPanel({ container, host, declaredFrameDomains, onStatus = () => {}, onContext = () => {}, onPage = () => {}, timeoutMS = 12000 }) {
  const document = container.ownerDocument;
  const window = document.defaultView;
  let frame;
  let currentURL;
  let channel;
  let timer;
  let ready = false;
  let suspended = false;
  let sessionStarted = false;
  let disposed = false;

  function status(state, message) { onStatus({ state, message }); }
  function clearTimer() { if (timer !== undefined) window.clearTimeout(timer); timer = undefined; }
  function reset() {
    clearTimer();
    ready = false;
    suspended = false;
    sessionStarted = false;
    channel = undefined;
    currentURL = undefined;
    if (frame) { frame.src = 'about:blank'; frame.remove(); frame = undefined; }
    onPage(null);
    onContext(null);
  }
  function unavailable(message) {
    // A failed navigation must not discard drafts in a previously ready viewer.
    if (sessionStarted) suspend();
    else reset();
    status('unavailable', message);
    return { embedded: false, reason: message };
  }
  function initialize() {
    if (!frame || !currentURL || suspended) return;
    ready = false;
    onContext(null);
    frame.contentWindow.postMessage({ protocol: 'openknowledge-viewer', version: 1, type: 'initialize', channel, url: currentURL.href }, currentURL.origin);
  }
  function suspend() {
    clearTimer();
    ready = false;
    suspended = true;
    if (frame) frame.hidden = true;
    onPage(null);
    onContext(null);
  }
  function receive(event) {
    const data = event.data;
    if (!frame || suspended || event.source !== frame.contentWindow || data?.protocol !== 'openknowledge-viewer' || data.version !== 1) return;
    if (event.origin === 'null') {
      unavailable('The host uses an opaque viewer sandbox. Open the local viewer to edit this page.');
      return;
    }
    if (event.origin !== currentURL.origin) return;
    if (data.type === 'loaded') { initialize(); return; }
    if (data.channel !== channel) return;
    if (data.type === 'ready') { ready = true; sessionStarted = true; clearTimer(); status('ready', 'Local viewer ready. Select a passage to share it with the conversation.'); }
    else if (ready && data.type === 'page') {
      const page = data.page;
      if (!page) { onPage(null); return; }
      if (typeof page.path !== 'string' || !page.path || page.path.length > 4096 || typeof page.knowledgeBase !== 'string' || page.knowledgeBase.length > 256) return;
      onPage({ path: page.path, knowledgeBase: page.knowledgeBase, title: typeof page.title === 'string' ? page.title.slice(0, 4096) : page.path });
    }
    else if (ready && data.type === 'context') {
      const context = data.context;
      if (!context) { onContext(null); return; }
      if (typeof context.text !== 'string' || !context.text.trim() || typeof context.path !== 'string' || context.path.length > 4096 || typeof context.knowledgeBase !== 'string' || context.knowledgeBase.length > 256) return;
      onContext({ text: context.text.slice(0, 24000), path: context.path, knowledgeBase: context.knowledgeBase,
        title: typeof context.title === 'string' ? context.title.slice(0, 4096) : context.path,
        selection: true, dirty: context.dirty === true, truncated: context.truncated === true || context.text.length > 24000, contentKind: 'rendered-selection' });
    }
  }
  function policyViolation(event) {
    if (frame && event.violatedDirective?.startsWith('frame-src') && event.blockedURI?.startsWith(currentURL.origin)) {
      unavailable('The host blocked the local viewer panel. Open the local viewer to read or edit this page.');
    }
  }
  window.addEventListener('message', receive);
  document.addEventListener('securitypolicyviolation', policyViolation);
  return {
    get ready() { return ready; },
    open(value) {
      if (disposed) throw new Error('The viewer panel is closed.');
      const url = normalizeEditorURL(value);
      if (!frameAllowedByHost(url.href, host, declaredFrameDomains)) return unavailable('This host does not permit the local viewer in its panel. Open the local viewer to read or edit this page.');
      const previousURL = currentURL;
      const reuse = frame && previousURL?.origin === url.origin && previousURL.pathname.split('/')[1] === url.pathname.split('/')[1];
      if (reuse) { clearTimer(); ready = false; onPage(null); onContext(null); }
      else reset();
      currentURL = url;
      suspended = false;
      channel = window.crypto?.randomUUID?.() || `${Date.now()}-${Math.random()}`;
      if (reuse) frame.hidden = false;
      else {
        frame = document.createElement('iframe');
        frame.className = 'viewer-frame';
        frame.title = 'Open Knowledge local viewer';
        frame.setAttribute('sandbox', 'allow-scripts allow-same-origin allow-forms');
        frame.setAttribute('referrerpolicy', 'no-referrer');
        frame.addEventListener('load', initialize);
        frame.src = url.href;
        container.append(frame);
      }
      status('loading', 'Opening the local viewer…');
      timer = window.setTimeout(() => unavailable('The local viewer did not become ready in this host. Open the local viewer to read or edit this page.'), timeoutMS);
      if (reuse) frame.contentWindow.postMessage({ protocol: 'openknowledge-viewer', version: 1, type: previousURL.href === url.href ? 'initialize' : 'navigate', channel, url: url.href, navigate: true }, url.origin);
      return { embedded: true };
    },
    requestContext() {
      if (ready) frame.contentWindow.postMessage({ protocol: 'openknowledge-viewer', version: 1, type: 'context-request', channel }, currentURL.origin);
    },
    suspend() { suspend(); status('suspended', 'Viewer hidden. Drafts remain open.'); },
    close() { reset(); status('closed', 'Viewer closed.'); },
    dispose() { reset(); disposed = true; window.removeEventListener('message', receive); document.removeEventListener('securitypolicyviolation', policyViolation); },
  };
}

// External handoff is explicit. Hosts decide where their link API displays the
// original viewer, so this API must not claim that it opens a side panel.
export async function openEditor(value, { host, openPreview, viewerPanel, panelURL }) {
  const url = normalizeEditorURL(value);
  if (viewerPanel && panelURL) return viewerPanel.open(panelURL);
  if (host) {
    if (!host.getHostCapabilities()?.openLinks) throw new Error('This host cannot open editor links. Open the local viewer in a browser.');
    const result = await host.openLink({ url: url.href });
    if (result?.isError) throw new Error('The host could not open the local viewer.');
  } else {
    if (typeof openPreview !== 'function') throw new Error('Open the local viewer in a browser.');
    await openPreview(url.href);
  }
}
