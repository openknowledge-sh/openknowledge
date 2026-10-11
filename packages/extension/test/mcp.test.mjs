import { test, mock } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { InMemoryTransport } from '@modelcontextprotocol/sdk/inMemory.js';
import { Knowledge, RegistryKnowledge, normalizeViewerURL } from '../server/knowledge.mjs';
import { createMCPServer, prepareAppResource, UI_URI } from '../server/mcp.mjs';
import { localRequestAllowed, options } from '../server/main.mjs';

test('MCP handshake exposes both entrypoints, returns the current UI and reads selected source', async () => {
  const dir = await mkdtemp(path.join(tmpdir(), 'okn-extension-'));
  const html = path.join(dir, 'app.html');
  await writeFile(html, '<h1>first build</h1>');
  const knowledge = {
    list: async () => ({ name: 'Test', pages: [{ path: 'note.md', title: 'Note' }] }),
    read: async pagePath => {
      if (pagePath !== 'note.md') throw new Error('Page is not in this knowledge base.');
      return { path: pagePath, title: 'Note', html: '<h1>Note</h1>', markdown: '# Note', sourceURI: 'openknowledge://bundle/note.md' };
    },
    search: async query => ({ content: [{ type: 'text', text: query }], structuredContent: { sources: [{ path: 'note.md' }] } }),
  };
  const server = createMCPServer(knowledge, html);
  const client = new Client({ name: 'test-client', version: '1' });
  const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
  try {
    await server.connect(serverTransport);
    await client.connect(clientTransport);
    const { tools } = await client.listTools();
    const library = tools.find(tool => tool.name === 'openknowledge_library');
    assert.deepEqual(library._meta['openai/ui'].entrypoints, [{ type: 'global' }, { type: 'thread' }]);
    assert.equal(library._meta.ui.resourceUri, UI_URI);
    assert.ok(tools.filter(tool => tool.name !== 'openknowledge_viewer_save').every(tool => tool.annotations.readOnlyHint));
    const save = tools.find(tool => tool.name === 'openknowledge_viewer_save');
    assert.equal(save.annotations.readOnlyHint, false);
    assert.deepEqual(save._meta.ui.visibility, ['app']);
    assert.deepEqual(tools.find(tool => tool.name === 'openknowledge_viewer_get')._meta.ui.visibility, ['app']);
    const opened = await client.callTool({ name: library.name, arguments: { path: 'note.md' } });
    assert.equal(opened.structuredContent.selectedPath, 'note.md');
    const page = await client.callTool({ name: 'openknowledge_read_page', arguments: { path: 'note.md' } });
    assert.match(page.content[0].text, /# Note/);
    const denied = await client.callTool({ name: 'openknowledge_read_page', arguments: { path: '../secret.md' } });
    assert.equal(denied.isError, true);
    const invalid = await client.callTool({ name: 'openknowledge_read_page', arguments: { path: 'note.md', root: '/tmp' } });
    assert.equal(invalid.isError, true);
    assert.match((await client.readResource({ uri: UI_URI })).contents[0].text, /first build/);
    await writeFile(html, '<h1>second build</h1>');
    assert.match((await client.readResource({ uri: UI_URI })).contents[0].text, /second build/);
  } finally { await client.close(); await server.close(); await rm(dir, { recursive: true, force: true }); }
});

test('inventory pagination and Markdown allowlist reject traversal before reading', async () => {
  let reads = 0;
  const upstream = {
    async listResources({ cursor } = {}) {
      return cursor ? { resources: [{ name: 'nested/note.md', uri: 'openknowledge://bundle/nested/note.md', title: 'Note' }] }
        : { resources: [{ name: 'secret.txt', uri: 'openknowledge://bundle/secret.txt' }], nextCursor: 'next' };
    },
    async readResource() { reads++; throw new Error('Must not be reached'); },
  };
  const knowledge = new Knowledge(upstream, 'http://127.0.0.1:1', 'Test');
  assert.deepEqual((await knowledge.list()).pages.map(page => page.path), ['nested/note.md']);
  await assert.rejects(knowledge.read('../secret.md'), /not in this knowledge base/);
  await assert.rejects(knowledge.read('nested/%2e%2e/secret.md'), /not in this knowledge base/);
  await assert.rejects(knowledge.read('secret.txt'), /not in this knowledge base/);
  assert.equal(reads, 0);
});

test('development access rejects cross-origin requests and DNS rebinding hosts', () => {
  assert.equal(localRequestAllowed({ headers: { host: '127.0.0.1:4318' } }), true);
  assert.equal(localRequestAllowed({ headers: { host: '127.0.0.1:4318', origin: 'http://127.0.0.1:4318' } }), true);
  assert.equal(localRequestAllowed({ headers: { host: '127.0.0.1:4318', origin: 'https://example.com' } }), false);
  assert.equal(localRequestAllowed({ headers: { host: 'wiki.attacker.example:4318' } }), false);
});

test('the editor handoff preserves the configured knowledge base prefix and inventory boundary', async () => {
  const pagePath = 'notes/A & B.md';
  let rendererURL;
  const upstream = {
    async listResources() { return { resources: [{ name: pagePath, uri: 'openknowledge://bundle/note', title: 'A & B' }] }; },
    async readResource() { return { contents: [{ text: '# A & B' }] }; },
  };
  const renderer = mock.method(globalThis, 'fetch', async url => {
    rendererURL = url.href;
    return { ok: true, json: async () => ({ body: '<h1>A &amp; B</h1>' }) };
  });
  try {
    const knowledge = new Knowledge(upstream, 'http://127.0.0.1:5176/docs', 'Docs');
    const page = await knowledge.read(pagePath);
    assert.equal(rendererURL, 'http://127.0.0.1:5176/docs/api/file/notes/A%20%26%20B.md');
    assert.equal(page.editorURL, 'http://127.0.0.1:5176/docs/file/notes/A%20%26%20B.md?edit=1');
    await assert.rejects(knowledge.read('../private.md'), /not in this knowledge base/);
    assert.equal(renderer.mock.callCount(), 1);
  } finally { renderer.mock.restore(); }
});

test('viewer overrides accept only loopback and do not accept credentials or URL query inputs', () => {
  assert.equal(normalizeViewerURL('http://localhost:5176/docs'), 'http://localhost:5176/docs/');
  assert.equal(options(['--viewer-url', 'http://127.0.0.1:5176/docs/']).viewerURL, 'http://127.0.0.1:5176/docs/');
  for (const url of ['https://example.com', 'http://example.com', 'file:///tmp/wiki', 'http://127.0.0.1@evil.example', 'http://user:secret@127.0.0.1', 'http://127.0.0.1/?q=1', 'http://127.0.0.1/#route']) {
    assert.throws(() => normalizeViewerURL(url), /loopback HTTP URL/);
  }
});

test('resource bootstrap is inert JSON before application code and binds CSP to its own viewer', async () => {
  const bootstrap = { viewerURL: 'http://127.0.0.1:5176/docs/</script><script>alert(1)</script>&\u2028\u2029', panelURL: 'http://127.0.0.1:5180/token/docs/' };
  const prepared = await prepareAppResource({
    getViewerBootstrap: async () => bootstrap,
    getFrameDomains: async () => { throw new Error('A different tool pool must not choose resource CSP.'); },
  }, '<!doctype html><html><head><script type="module">start()</script></head></html>');
  const match = prepared.html.match(/<script type="application\/json" id="openknowledge-viewer-bootstrap">([\s\S]*?)<\/script>/);
  assert.ok(match);
  assert.deepEqual(JSON.parse(match[1]), bootstrap);
  assert.ok(!/[<>&\u2028\u2029]/.test(match[1]), 'JSON cannot close its inert script or create HTML');
  assert.ok(prepared.html.indexOf('id="openknowledge-viewer-bootstrap"') < prepared.html.indexOf('type="module"'));
  assert.deepEqual(prepared.frameDomains, ['http://127.0.0.1:5180']);
  assert.deepEqual(prepared.bootstrap, bootstrap);
  const legacy = await prepareAppResource({ getFrameDomains: async () => ['http://127.0.0.1:5190'] }, '<h1>Legacy fixture</h1>');
  assert.equal(legacy.html, '<h1>Legacy fixture</h1>');
  assert.equal(legacy.bootstrap, undefined);
  assert.deepEqual(legacy.frameDomains, ['http://127.0.0.1:5190']);
});

test('MCP scopes tool calls by registry alias and declares its viewer panel origin', async () => {
  const dir = await mkdtemp(path.join(tmpdir(), 'okn-extension-registry-'));
  const html = path.join(dir, 'app.html');
  await writeFile(html, '<h1>Registry</h1>');
  const upstream = alias => ({
    listResources: async () => ({ resources: [{ name: 'note.md', uri: 'openknowledge://bundle/note.md' }] }),
    readResource: async () => ({ contents: [{ text: `# ${alias}` }] }),
    callTool: async () => ({ content: [], structuredContent: { sources: [{ path: 'note.md' }] } }),
  });
  const knowledge = new RegistryKnowledge({
    loadRegistry: async () => [{ name: 'work', path: '/work', access: 'write' }, { name: 'personal', path: '/personal', access: 'read' }],
    openBase: async (entry, viewer) => ({ knowledge: new Knowledge(upstream(entry.name), viewer, entry.name, { access: entry.access }), close: async () => {} }),
    startViewer: async () => ({ url: 'http://127.0.0.1:5176/', getPanel: async () => ({ origin: 'http://127.0.0.1:5180', urlFor: value => `http://127.0.0.1:5180/token${new URL(value).pathname}` }), close: async () => {} }),
  });
  const server = createMCPServer(knowledge, html, { native: false });
  const client = new Client({ name: 'registry-client', version: '1' });
  const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
  const renderer = mock.method(globalThis, 'fetch', async () => ({ ok: true, json: async () => ({ body: '<p>Note</p>' }) }));
  try {
    await server.connect(serverTransport);
    await client.connect(clientTransport);
    const home = await client.callTool({ name: 'openknowledge_library', arguments: {} });
    assert.equal(home.structuredContent.viewerURL, 'http://127.0.0.1:5176/');
    assert.equal(home.structuredContent.panelURL, 'http://127.0.0.1:5180/token/');
    assert.deepEqual(home.structuredContent.knowledgeBases.map(base => base.alias), ['work', 'personal']);
    const selected = await client.callTool({ name: 'openknowledge_library', arguments: { knowledgeBase: 'personal', path: 'note.md' } });
    assert.equal(selected.structuredContent.selectedKnowledgeBase, 'personal');
    assert.equal(selected.structuredContent.selectedPath, 'note.md');
    const missing = await client.callTool({ name: 'openknowledge_read_page', arguments: { path: 'note.md' } });
    assert.equal(missing.isError, true);
    const unknown = await client.callTool({ name: 'openknowledge_read_page', arguments: { knowledgeBase: 'missing', path: 'note.md' } });
    assert.equal(unknown.isError, true);
    const page = await client.callTool({ name: 'openknowledge_read_page', arguments: { knowledgeBase: 'personal', path: 'note.md' } });
    assert.equal(page.structuredContent.knowledgeBase, 'personal');
    assert.equal(page.structuredContent.editorURL, undefined);
    assert.match(page.content[0].text, /Knowledge base: personal/);
    const result = await client.readResource({ uri: UI_URI });
    assert.deepEqual(result.contents[0]._meta.ui.csp.frameDomains, ['http://127.0.0.1:5180']);
    const bootstrap = JSON.parse(result.contents[0].text.match(/id="openknowledge-viewer-bootstrap">([\s\S]*?)<\/script>/)[1]);
    assert.equal(bootstrap.panelURL, home.structuredContent.panelURL);
    assert.equal(bootstrap.viewerURL, home.structuredContent.viewerURL);
  } finally { renderer.mock.restore(); await client.close(); await server.close(); await knowledge.close(); await rm(dir, { recursive: true, force: true }); }
});
