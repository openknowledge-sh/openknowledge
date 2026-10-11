import { test } from 'node:test';
import assert from 'node:assert/strict';
import { openEditor, createViewerPanel, frameAllowedByHost } from '../web/editor-link.mjs';

const editorURL = 'http://127.0.0.1:5176/docs/file/note.md?edit=1';

test('editor handoff uses the negotiated host link API and preserves the viewer URL', async () => {
  let received;
  await openEditor(editorURL, {
    host: { getHostCapabilities: () => ({ openLinks: {} }), openLink: async input => { received = input; } },
    openPreview: () => assert.fail('Native hosts must receive the link through their API'),
  });
  assert.deepEqual(received, { url: editorURL });
});

test('local preview opens the same editor and missing host support produces an actionable error', async () => {
  let received;
  await openEditor(editorURL, { openPreview: url => { received = url; } });
  assert.equal(received, editorURL);
  await assert.rejects(openEditor(editorURL, { host: { getHostCapabilities: () => ({}) } }), /host cannot open editor links/);
});

test('editor handoff rejects nonlocal URLs before any host operation', async () => {
  const host = { getHostCapabilities: () => assert.fail('Invalid URLs cannot reach host') };
  for (const url of ['https://example.com', 'javascript:alert(1)', 'file:///tmp/wiki', 'http://127.0.0.1.evil.example', 'http://user@127.0.0.1']) {
    await assert.rejects(openEditor(url, { host }), /local Open Knowledge viewer/);
  }
});

function panelHarness(capabilities, declaredFrameDomains) {
  class Surface {
    listeners = new Map();
    addEventListener(type, listener) { this.listeners.set(type, listener); }
    removeEventListener(type, listener) { if (this.listeners.get(type) === listener) this.listeners.delete(type); }
    dispatch(type, event) { this.listeners.get(type)?.(event); }
  }
  const window = new Surface();
  let channels = 0;
  window.crypto = { randomUUID: () => ++channels === 1 ? 'test-channel' : `test-channel-${channels}` };
  window.setTimeout = callback => { window.timeout = callback; return 1; };
  window.clearTimeout = () => { window.timeout = undefined; };
  const document = new Surface();
  document.defaultView = window;
  let frame;
  document.createElement = () => {
    frame = new Surface();
    frame.attributes = {};
    frame.setAttribute = (name, value) => { frame.attributes[name] = value; };
    frame.contentWindow = { postMessage: (data, origin) => { frame.sent = { data, origin }; } };
    frame.remove = () => { frame.removed = true; };
    return frame;
  };
  const container = { ownerDocument: document, append: () => {} };
  const statuses = [];
  const contexts = [];
  const pages = [];
  const panel = createViewerPanel({ container, declaredFrameDomains, host: capabilities === undefined ? undefined : { getHostCapabilities: () => capabilities }, onStatus: status => statuses.push(status), onContext: context => contexts.push(context), onPage: page => pages.push(page) });
  const receive = (data, origin = 'http://127.0.0.1:5176', source = frame.contentWindow) => window.dispatch('message', { source, origin, data: { protocol: 'openknowledge-viewer', version: 1, channel: frame.sent?.data.channel || 'test-channel', ...data } });
  return { panel, window, document, statuses, contexts, pages, receive, get frame() { return frame; } };
}

test('viewer panels respect explicit host CSP reports and resource declarations', () => {
  assert.equal(frameAllowedByHost(editorURL, undefined), true);
  assert.equal(frameAllowedByHost(editorURL, { getHostCapabilities: () => ({ openLinks: {} }) }), false);
  assert.equal(frameAllowedByHost(editorURL, { getHostCapabilities: () => ({ sandbox: { csp: { frameDomains: ['http://127.0.0.1:5176'] } } }) }), true);
  assert.equal(frameAllowedByHost(editorURL, { getHostCapabilities: () => ({ sandbox: { csp: { frameDomains: ['http://127.0.0.1:5177'] } } }) }), false);
  const declared = ['http://127.0.0.1:5176'];
  for (const frameDomains of [[], ['http://127.0.0.1:5177'], null, '*', {}, ['http://127.0.0.1:5176', 1]]) {
    assert.equal(frameAllowedByHost(editorURL, { getHostCapabilities: () => ({ sandbox: { csp: { frameDomains } } }) }, declared), false);
  }
  assert.equal(frameAllowedByHost(editorURL, { getHostCapabilities: () => ({ sandbox: { csp: { frameDomains: declared } } }) }, ['http://127.0.0.1:5177']), false);
  const denied = panelHarness({ openLinks: {} });
  assert.equal(denied.panel.open(editorURL).embedded, false);
  assert.equal(denied.frame, undefined);
  assert.match(denied.statuses.at(-1).message, /does not permit/);
  denied.panel.dispose();
});

test('an omitted optional host CSP report permits only a declared resource origin to be probed', () => {
  const declared = ['http://127.0.0.1:5176'];
  for (const capabilities of [undefined, {}, { sandbox: {} }, { sandbox: { csp: {} } }]) {
    const host = { getHostCapabilities: () => capabilities };
    assert.equal(frameAllowedByHost(editorURL, host, declared), true);
    assert.equal(frameAllowedByHost(editorURL, host), false);
    assert.equal(frameAllowedByHost(editorURL, host, []), false);
    assert.equal(frameAllowedByHost(editorURL.replace(':5176', ':5177'), host, declared), false);
  }
  for (const capabilities of [null, [], 'invalid', { sandbox: null }, { sandbox: [] }, { sandbox: { csp: null } }, { sandbox: { csp: [] } }]) {
    assert.equal(frameAllowedByHost(editorURL, { getHostCapabilities: () => capabilities }, declared), false);
  }
  assert.equal(frameAllowedByHost(editorURL, { getHostCapabilities: () => ({}) }, declared[0]), false);
});

test('missing capability reporting does not override enforced iframe CSP or establish readiness', () => {
  const declared = ['http://127.0.0.1:5176'];
  for (const mode of ['policy', 'timeout']) {
    const env = panelHarness({ openLinks: {} }, declared);
    assert.equal(env.panel.open(editorURL).embedded, true);
    env.frame.dispatch('load');
    assert.equal(env.panel.ready, false, 'only a trusted ready message proves a usable viewer');
    if (mode === 'policy') env.document.dispatch('securitypolicyviolation', { violatedDirective: 'frame-src', blockedURI: editorURL });
    else env.window.timeout();
    assert.equal(env.panel.ready, false);
    assert.equal(env.frame.removed, true);
    assert.equal(env.statuses.at(-1).state, 'unavailable');
    assert.match(env.statuses.at(-1).message, mode === 'policy' ? /host blocked/ : /did not become ready/);
    env.panel.dispose();
  }
});

test('panel ready requires the trusted viewer bridge and context cannot be spoofed by another frame', () => {
  const env = panelHarness();
  assert.equal(env.panel.open(editorURL).embedded, true);
  env.frame.dispatch('load');
  assert.equal(env.panel.ready, false, 'onload is not proof of a usable viewer');
  assert.equal(env.frame.sent.origin, 'http://127.0.0.1:5176');
  assert.equal(env.frame.attributes.sandbox, 'allow-scripts allow-same-origin allow-forms');
  env.receive({ type: 'ready' }, 'https://example.com');
  env.receive({ type: 'ready' }, 'http://127.0.0.1:5176', {});
  assert.equal(env.panel.ready, false);
  env.receive({ type: 'ready' });
  assert.equal(env.panel.ready, true);
  env.receive({ type: 'page', page: { path: 'note.md', knowledgeBase: 'test', title: 'Note' } });
  assert.deepEqual(env.pages.at(-1), { path: 'note.md', knowledgeBase: 'test', title: 'Note' });
  env.receive({ type: 'page', page: { path: 'ignored.md', knowledgeBase: { unsafe: true } } });
  assert.equal(env.pages.at(-1).path, 'note.md');
  const context = { text: 'Changed draft passage', title: 'Draft', path: 'note.md', knowledgeBase: 'test', selection: true, dirty: true, revision: 'stale-saved-revision' };
  env.receive({ type: 'context', context });
  assert.equal(env.contexts.at(-1).dirty, true);
  assert.equal(env.contexts.at(-1).contentKind, 'rendered-selection');
  assert.equal(env.contexts.at(-1).revision, undefined, 'rendered draft selection must not claim the saved source revision');
  env.receive({ type: 'context', context: { ...context, text: 'x'.repeat(24001) } });
  assert.equal(env.contexts.at(-1).text.length, 24000);
  assert.equal(env.contexts.at(-1).truncated, true);
  env.panel.close();
  assert.equal(env.pages.at(-1), null);
  assert.equal(env.contexts.at(-1), null);
  assert.equal(env.frame.removed, true);
  env.panel.dispose();
  assert.equal(env.window.listeners.has('message'), false);
});

test('viewer hiding and page changes reuse the live frame and renew the message channel', () => {
  const env = panelHarness();
  env.panel.open(editorURL);
  const originalFrame = env.frame;
  env.frame.dispatch('load');
  env.receive({ type: 'ready' });
  env.panel.suspend();
  assert.equal(originalFrame.hidden, true);
  assert.equal(originalFrame.removed, undefined);
  assert.equal(env.panel.ready, false);
  env.receive({ type: 'page', page: { path: 'hidden.md', knowledgeBase: 'test' } });
  assert.equal(env.pages.at(-1), null);
  env.panel.open(editorURL);
  assert.equal(env.frame, originalFrame);
  assert.equal(env.frame.hidden, false);
  assert.equal(env.frame.sent.data.type, 'initialize');
  assert.equal(env.frame.sent.data.channel, 'test-channel-2');
  env.receive({ type: 'ready', channel: 'test-channel' });
  assert.equal(env.panel.ready, false, 'a previous selection cannot reactivate the resumed viewer');
  env.receive({ type: 'ready' });
  const nextURL = 'http://127.0.0.1:5176/docs/file/other.md';
  env.panel.open(nextURL);
  assert.equal(env.frame, originalFrame);
  assert.equal(env.frame.src, editorURL, 'SPA navigation leaves the frame document alive');
  assert.equal(env.frame.sent.data.type, 'navigate');
  assert.equal(env.frame.sent.data.url, nextURL);
  assert.equal(env.frame.sent.data.channel, 'test-channel-3');
  env.window.timeout();
  assert.equal(originalFrame.removed, undefined, 'failed navigation preserves the existing drafts');
  assert.equal(originalFrame.hidden, true);
  env.panel.dispose();
  assert.equal(originalFrame.removed, true);
});

test('opaque or unresponsive host sandboxes fail visibly and dispose the inactive viewer', () => {
  for (const mode of ['opaque', 'timeout']) {
    const env = panelHarness();
    env.panel.open(editorURL);
    if (mode === 'opaque') env.receive({ type: 'loaded' }, 'null');
    else env.window.timeout();
    assert.equal(env.panel.ready, false);
    assert.equal(env.frame.removed, true);
    assert.equal(env.statuses.at(-1).state, 'unavailable');
    env.panel.dispose();
  }
});

test('external viewer handoff reports host link refusal', async () => {
  await assert.rejects(openEditor(editorURL, { host: { getHostCapabilities: () => ({ openLinks: {} }), openLink: async () => ({ isError: true }) } }), /could not open/);
});
