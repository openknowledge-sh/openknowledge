import { test, mock } from 'node:test';
import assert from 'node:assert/strict';
import { Knowledge, RegistryKnowledge, registryMetadata, loadRegistry } from '../server/knowledge.mjs';
import { callKnowledgeTool } from '../server/mcp.mjs';
import { options } from '../server/main.mjs';
import { mkdtemp, writeFile, chmod, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';

const entry = (name, access = 'write') => ({ name, path: `/bases/${name}`, access });
const resource = name => ({ name, uri: `openknowledge://bundle/${name}`, title: 'Shared note' });

function fixture(initial = [entry('work'), entry('personal', 'read')]) {
  let entries = initial;
  const opened = [];
  const closed = [];
  let viewerStarts = 0;
  let viewerCloses = 0;
  const knowledge = new RegistryKnowledge({
    loadRegistry: async () => entries,
    openBase: async (base, viewer, panel) => {
      opened.push(base.name);
      if (base.path === '/missing') throw new Error('The local root does not exist.');
      const client = {
        listResources: async () => ({ resources: [resource('note.md')] }),
        readResource: async () => ({ contents: [{ text: `# ${base.name}` }] }),
        callTool: async () => ({ content: [{ type: 'text', text: base.name }], structuredContent: { sources: [{ path: 'note.md', text: base.name }] } }),
      };
      return { knowledge: new Knowledge(client, viewer, base.name, registryMetadata(base), panel), close: async () => closed.push(base.name) };
    },
    startViewer: async () => {
      viewerStarts++;
      return { url: 'http://127.0.0.1:5176/', getPanel: async () => ({ origin: 'http://127.0.0.1:5180', urlFor: value => 'http://127.0.0.1:5180/token' + new URL(value).pathname + new URL(value).search }), close: async () => viewerCloses++ };
    },
  });
  return { knowledge, opened, closed, replace(value) { entries = value; }, get viewerStarts() { return viewerStarts; }, get viewerCloses() { return viewerCloses; } };
}

test('registry is the default and --root preserves explicit single-base mode', () => {
  assert.equal(options([]).root, undefined);
  assert.equal(options(['--root', '.']).root, process.cwd());
});

test('full-viewer home starts lazily and rejects unregistered or unlisted destinations before viewer startup', async () => {
  const runtime = fixture();
  try {
    assert.equal(runtime.viewerStarts, 0);
    await assert.rejects(runtime.knowledge.library('unknown'), /Unknown knowledge base/);
    await assert.rejects(runtime.knowledge.library(undefined, 'note.md'), /knowledge base is required/);
    await assert.rejects(runtime.knowledge.library('personal', '../note.md'), /not in this knowledge base/);
    assert.equal(runtime.viewerStarts, 0);
    const inventory = await runtime.knowledge.list('work');
    assert.equal(inventory.pages[0].knowledgeBase, 'work');
    assert.equal(runtime.viewerStarts, 0, 'MCP inventory alone does not start a viewer');
    const home = await runtime.knowledge.library();
    const base = await runtime.knowledge.library('personal');
    assert.equal(home.viewerURL, 'http://127.0.0.1:5176/');
    assert.equal(home.panelURL, 'http://127.0.0.1:5180/token/');
    assert.equal(base.viewerURL, 'http://127.0.0.1:5176/kb/personal/');
    assert.equal(base.panelURL, 'http://127.0.0.1:5180/token/kb/personal/');
    assert.equal(runtime.viewerStarts, 1);
    assert.deepEqual(await runtime.knowledge.getFrameDomains(), ['http://127.0.0.1:5180']);
    assert.equal(runtime.viewerStarts, 1);
  } finally { await runtime.knowledge.close(); }
});

test('explicit root home keeps its viewer alias prefix and full panel without widening the page inventory', async () => {
  const requested = [];
  let viewerLoads = 0;
  const upstream = { listResources: async () => ({ resources: [resource('notes/A & B.md')] }) };
  const panel = { origin: 'http://127.0.0.1:5180', urlFor(value, projection) {
    assert.equal(projection, undefined, 'the original full viewer is the default projection');
    requested.push(value);
    return 'http://127.0.0.1:5180/token' + new URL(value).pathname;
  } };
  const knowledge = new Knowledge(upstream, () => { viewerLoads++; return 'http://127.0.0.1:5176/docs/'; }, 'docs', { access: 'read' }, async () => panel);
  await assert.rejects(knowledge.library('outside'), /Unknown knowledge base/);
  await assert.rejects(knowledge.library('docs', '../private.md'), /not in this knowledge base/);
  assert.equal(viewerLoads, 0);
  const home = await knowledge.library(undefined, 'notes/A & B.md');
  assert.equal(home.mode, 'root');
  assert.equal(home.viewerURL, 'http://127.0.0.1:5176/docs/');
  assert.equal(home.panelURL, 'http://127.0.0.1:5180/token/docs/');
  assert.equal(home.selectedKnowledgeBase, 'docs');
  assert.equal(home.selectedPath, 'notes/A & B.md');
  assert.deepEqual(requested, ['http://127.0.0.1:5176/docs/']);
  assert.equal(home.knowledgeBases[0].access, 'read');
});

test('resource bootstrap starts only its cached viewer and preserves an explicit root prefix', async () => {
  let starts = 0;
  const panel = { origin: 'http://127.0.0.1:5180', urlFor: value => 'http://127.0.0.1:5180/token' + new URL(value).pathname };
  const registry = new RegistryKnowledge({
    loadRegistry: async () => { throw new Error('Bootstrap must not enumerate the registry.'); },
    openBase: async () => { throw new Error('Bootstrap must not start a knowledge base MCP.'); },
    startViewer: async () => { starts++; return { url: 'http://127.0.0.1:5176/', getPanel: async () => panel, close: async () => {} }; },
  });
  try {
    const bootstrap = await registry.getViewerBootstrap();
    assert.deepEqual(bootstrap, { viewerURL: 'http://127.0.0.1:5176/', panelURL: 'http://127.0.0.1:5180/token/' });
    assert.deepEqual(await registry.getViewerBootstrap(), bootstrap);
    assert.deepEqual(await registry.getFrameDomains(), [new URL(bootstrap.panelURL).origin]);
    assert.equal(starts, 1);
    const root = new Knowledge({ listResources: async () => { throw new Error('Bootstrap must not enumerate documents.'); } }, 'http://127.0.0.1:5176/docs/', 'docs', {}, async () => panel);
    assert.deepEqual(await root.getViewerBootstrap(), { viewerURL: 'http://127.0.0.1:5176/docs/', panelURL: 'http://127.0.0.1:5180/token/docs/' });
  } finally { await registry.close(); }
});

test('library starts one full viewer while inventory keeps unavailable connections without starting their MCP', async () => {
  const runtime = fixture([entry('work'), { ...entry('missing', 'read'), path: '/missing', managed: true, source: { type: 'git', url: 'https://example.test/wiki.git' } }]);
  try {
    const home = await callKnowledgeTool(runtime.knowledge, 'openknowledge_library', {});
    assert.equal(home.structuredContent.mode, 'registry');
    assert.deepEqual(home.structuredContent.pages, []);
    assert.deepEqual(home.structuredContent.knowledgeBases.map(base => base.alias), ['work', 'missing']);
    assert.equal(home.structuredContent.knowledgeBases[1].managed, true);
    assert.equal(home.structuredContent.knowledgeBases[1].source.type, 'git');
    assert.equal(home.structuredContent.viewerURL, 'http://127.0.0.1:5176/');
    assert.equal(home.structuredContent.panelURL, 'http://127.0.0.1:5180/token/');
    assert.deepEqual(runtime.opened, []);
    assert.equal(runtime.viewerStarts, 1);
    await assert.rejects(runtime.knowledge.library('unknown'), /Unknown knowledge base/);
    await assert.rejects(runtime.knowledge.library('missing'), /local root/);
    assert.equal((await runtime.knowledge.library()).knowledgeBases.length, 2);
    assert.equal(runtime.viewerStarts, 1);
    await assert.rejects(runtime.knowledge.list(), /Select a knowledge base/);
    await assert.rejects(runtime.knowledge.read('note.md'), /Select a knowledge base/);
    await assert.rejects(runtime.knowledge.library(undefined, 'note.md'), /knowledge base is required/);
  } finally { await runtime.knowledge.close(); }
});

test('duplicate document paths remain isolated by alias and use one shared viewer', async () => {
  const runtime = fixture();
  const urls = [];
  const fetch = mock.method(globalThis, 'fetch', async url => {
    urls.push(url.href);
    return { ok: true, json: async () => ({ body: '<h1>Shared note</h1>' }) };
  });
  try {
    const work = await runtime.knowledge.library('work', 'note.md');
    assert.equal(work.selectedKnowledgeBase, 'work');
    assert.equal(work.selectedPath, 'note.md');
    assert.equal(work.pages[0].knowledgeBase, 'work');
    assert.deepEqual(runtime.opened, ['work']);
    assert.equal(work.viewerURL, 'http://127.0.0.1:5176/kb/work/');
    assert.equal(work.panelURL, 'http://127.0.0.1:5180/token/kb/work/');
    assert.equal(runtime.viewerStarts, 1);
    await assert.rejects(runtime.knowledge.read('../secret.md', 'work'), /not in this knowledge base/);
    assert.equal(runtime.viewerStarts, 1);
    const writable = await runtime.knowledge.read('note.md', 'work');
    const readonly = await runtime.knowledge.read('note.md', 'personal');
    assert.equal(writable.markdown, '# work');
    assert.equal(readonly.markdown, '# personal');
    assert.equal(writable.knowledgeBase, 'work');
    assert.equal(readonly.knowledgeBase, 'personal');
    assert.equal(writable.viewerURL, 'http://127.0.0.1:5176/kb/work/file/note.md');
    assert.equal(writable.editorURL, 'http://127.0.0.1:5176/kb/work/file/note.md?edit=1');
    assert.equal(writable.panelURL, 'http://127.0.0.1:5180/token/kb/work/file/note.md');
    assert.equal(writable.panelEditorURL, 'http://127.0.0.1:5180/token/kb/work/file/note.md?edit=1');
    assert.equal(readonly.viewerURL, 'http://127.0.0.1:5176/kb/personal/file/note.md');
    assert.equal(readonly.editorURL, undefined);
    assert.equal(readonly.panelEditorURL, undefined);
    assert.equal(runtime.viewerStarts, 1);
    assert.deepEqual(urls, ['http://127.0.0.1:5176/kb/work/api/file/note.md', 'http://127.0.0.1:5176/kb/personal/api/file/note.md']);
    const search = await runtime.knowledge.search('note');
    assert.deepEqual(search.structuredContent.sources.map(source => [source.knowledgeBase, source.path]), [['work', 'note.md'], ['personal', 'note.md']]);
  } finally { fetch.mock.restore(); await runtime.knowledge.close(); }
  assert.deepEqual(runtime.closed.sort(), ['personal', 'work']);
  assert.equal(runtime.viewerCloses, 1);
  await runtime.knowledge.close();
  assert.equal(runtime.viewerCloses, 1);
  await assert.rejects(runtime.knowledge.library(), /library is closed/);
});

test('changed access or disconnected alias closes obsolete MCP leases before reuse', async () => {
  const runtime = fixture();
  const fetch = mock.method(globalThis, 'fetch', async () => ({ ok: true, json: async () => ({ body: '<p>Note</p>' }) }));
  try {
    await runtime.knowledge.library('work');
    runtime.replace([entry('work', 'read')]);
    const page = await runtime.knowledge.read('note.md', 'work');
    assert.equal(page.editorURL, undefined);
    assert.deepEqual(runtime.opened, ['work', 'work']);
    assert.deepEqual(runtime.closed, ['work']);
    runtime.replace([]);
    assert.deepEqual((await runtime.knowledge.library()).knowledgeBases, []);
    assert.deepEqual(runtime.closed, ['work', 'work']);
    await assert.rejects(runtime.knowledge.read('note.md', 'work'), /Unknown knowledge base/);
  } finally { fetch.mock.restore(); await runtime.knowledge.close(); }
});

test('concurrent selection starts one upstream and shutdown closes pending startup', async () => {
  let release;
  let starts = 0;
  let closes = 0;
  const pending = new Promise(resolve => { release = resolve; });
  const knowledge = new RegistryKnowledge({
    loadRegistry: async () => [entry('work')],
    openBase: async () => {
      starts++;
      await pending;
      return { knowledge: { list: async () => ({ pages: [] }) }, close: async () => { closes++; } };
    },
    startViewer: async () => { throw new Error('Viewer should stay lazy.'); },
  });
  const one = knowledge.list('work');
  const two = knowledge.list('work');
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(starts, 1);
  const stopped = knowledge.close();
  release();
  await Promise.all([assert.rejects(one, /closed/), assert.rejects(two, /closed/), stopped]);
  assert.equal(closes, 1);
});

test('shutdown waits for a disconnected upstream already being retired', async () => {
  let entries = [entry('work')];
  let release;
  let closes = 0;
  const pendingClose = new Promise(resolve => { release = resolve; });
  const knowledge = new RegistryKnowledge({
    loadRegistry: async () => entries,
    openBase: async () => ({ knowledge: { list: async () => ({ pages: [] }) }, close: async () => { closes++; await pendingClose; } }),
    startViewer: async () => { throw new Error('Viewer should stay lazy.'); },
  });
  await knowledge.list('work');
  entries = [];
  const refresh = knowledge.library();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(closes, 1);
  let stopped = false;
  const stop = knowledge.close().then(() => { stopped = true; });
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(stopped, false);
  release();
  await Promise.all([assert.rejects(refresh, /closed/), stop]);
  assert.equal(closes, 1);
});

test('global search keeps available results and reports failures without hiding their cards', async () => {
  const runtime = fixture([entry('work'), { ...entry('missing'), path: '/missing' }]);
  try {
    const searched = await runtime.knowledge.search('knowledge');
    assert.equal(searched.structuredContent.sources[0].knowledgeBase, 'work');
    assert.deepEqual(searched.structuredContent.errors, [{ knowledgeBase: 'missing', error: 'The local root does not exist.' }]);
    assert.equal((await runtime.knowledge.library()).knowledgeBases.length, 2);
    assert.equal(runtime.viewerStarts, 1);
  } finally { await runtime.knowledge.close(); }
});

test('managed sources always stay read-only even with a legacy write access value', () => {
  for (const source of [{ type: 'git' }, { url: 'https://example.test/wiki' }, { managedRoot: '/cache/wiki' }]) {
    assert.equal(registryMetadata({ ...entry('managed'), source }).access, 'read');
  }
  assert.equal(registryMetadata({ ...entry('managed'), managed: true }).access, 'read');
});

test('registry loader delegates to the configured CLI versioned inventory', async () => {
  const dir = await mkdtemp(path.join(tmpdir(), 'okn-registry-cli-'));
  const binary = path.join(dir, 'okn');
  await writeFile(binary, '#!/usr/bin/env node\nif (JSON.stringify(process.argv.slice(2)) !== JSON.stringify(["registry", "list", "--json"])) process.exit(2);\nconsole.log(JSON.stringify({schemaVersion:"1", entries:[{name:"from-cli", path:"/unavailable", access:"read"}]}));\n');
  await chmod(binary, 0o755);
  try {
    assert.deepEqual(await loadRegistry({ binary, repoRoot: dir }), [{ name: 'from-cli', path: '/unavailable', access: 'read' }]);
  } finally { await rm(dir, { recursive: true, force: true }); }
});
