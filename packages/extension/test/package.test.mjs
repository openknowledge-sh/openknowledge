import { test } from 'node:test';
import assert from 'node:assert/strict';
import { chmod, cp, mkdir, mkdtemp, readFile, rm, stat, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';
import { packageLocal, localMCPConfig } from '../scripts/package-local.mjs';

const expand = (value, plugin) => value.replaceAll('${PLUGIN_ROOT}', plugin);
const resourceBootstrap = resource => JSON.parse(resource.contents[0].text.match(/id="openknowledge-native-viewer-bootstrap">([\s\S]*?)<\/script>/)[1]);

test('local package config uses host-owned stdio and a plugin-relative executable', () => {
  const config = localMCPConfig();
  const server = config.mcpServers.openknowledge;
  assert.equal(server.type, 'stdio');
  assert.equal(server.command, process.platform === 'win32' ? './bin/node.exe' : './bin/node');
  assert.deepEqual(server.args, ['${PLUGIN_ROOT}/server/main.mjs', '--stdio']);
  assert.ok(server.env.OPENKNOWLEDGE_BIN.startsWith('${PLUGIN_ROOT}/bin/'));
  assert.equal(server.url, undefined);
});

test('isolated packaged stdio supports cold resource-first launch, EOF cleanup and restart', { timeout: 90000 }, async () => {
  const dir = await mkdtemp(path.join(tmpdir(), 'okn-extension-package-'));
  const buildRoot = path.join(dir, 'build');
  const isolatedRoot = path.join(dir, 'isolated');
  const knowledgeRoot = path.join(dir, 'knowledge');
  const readOnlyRoot = path.join(dir, 'read-only');
  const registry = path.join(dir, 'registry.json');
  const fixtureBinary = path.join(dir, 'fixture-cli');
  await mkdir(knowledgeRoot);
  await mkdir(readOnlyRoot);
  await writeFile(path.join(knowledgeRoot, 'index.md'), '---\ntype: Note\ntitle: Packaged note\n---\n\n# Packaged note\n');
  const readOnlyMarkdown = '---\ntype: Note\ntitle: Read only\n---\n\n# Read only\n';
  await writeFile(path.join(readOnlyRoot, 'index.md'), readOnlyMarkdown);
  await writeFile(registry, JSON.stringify({ schemaVersion: '1', connections: { [knowledgeRoot]: { key: 'packaged', access: 'write' }, [readOnlyRoot]: { key: 'read-only', access: 'read' } } }));
  // The default fixture isolates packaging from the Go build. Set
  // OPENKNOWLEDGE_TEST_CLI to repeat this lifecycle with a real CLI executable.
  await writeFile(fixtureBinary, `#!${process.execPath}\nconst {createServer}=require('node:http');\nconst fs=require('node:fs');\nif(process.argv[2]==='registry'){const r=JSON.parse(fs.readFileSync(process.env.OPENKNOWLEDGE_REGISTRY_FILE));console.log(JSON.stringify({schemaVersion:'1',entries:Object.entries(r.connections).map(([path,v])=>({name:v.key,path,access:v.access}))}));}\nelse if(process.argv[2]==='view'){const s=createServer((q,r)=>{r.writeHead(200,{'content-type':'text/html'});r.end('<html><head></head><body>Packaged fixture viewer</body></html>');});s.listen(0,'127.0.0.1',()=>console.log('Open Knowledge view: http://127.0.0.1:'+s.address().port+'/'));process.once('SIGTERM',()=>s.close(()=>process.exit(0)));}\nelse process.exit(2);\n`);
  await chmod(fixtureBinary, 0o755);
  let client;
  let peer;
  try {
    const built = await packageLocal({ destination: buildRoot, binary: process.env.OPENKNOWLEDGE_TEST_CLI || fixtureBinary });
    await cp(built.plugin, isolatedRoot, { recursive: true });
    await rm(buildRoot, { recursive: true, force: true });
    const config = JSON.parse(await readFile(path.join(isolatedRoot, 'mcp.json'), 'utf8')).mcpServers.openknowledge;
    const packagedNode = path.resolve(isolatedRoot, config.command);
    assert.ok((await stat(packagedNode)).isFile());
    assert.ok(packagedNode.startsWith(isolatedRoot + path.sep));
    const html = await readFile(path.join(isolatedRoot, 'dist/app.html'), 'utf8');
    assert.match(html, /Open Knowledge/);
    assert.ok(!html.includes('src="/app.js"'));
    const backend = await readFile(path.join(isolatedRoot, 'server/main.mjs'), 'utf8');
    assert.ok(!backend.includes('http://127.0.0.1:4318/mcp'));
    for (let round = 0; round < 2; round++) {
      const transport = new StdioClientTransport({
        // Codex resolves a contained command against PLUGIN_ROOT. Resolve that
        // command explicitly for the generic SDK, with cwd outside the plugin.
        command: packagedNode,
        args: config.args.map(value => expand(value, isolatedRoot)),
        env: { OPENKNOWLEDGE_REGISTRY_FILE: registry, ...Object.fromEntries(Object.entries(config.env).map(([key, value]) => [key, expand(value, isolatedRoot)])) },
        cwd: dir, stderr: 'pipe',
      });
      let stderr = '';
      transport.stderr.on('data', chunk => { stderr += chunk; });
      client = new Client({ name: 'packaged-cold-test', version: '1' });
      try { await client.connect(transport); } catch (error) { error.message += `\nPackaged stderr: ${stderr}`; throw error; }
      // Native sidebar launches can read the UI before invoking any tool.
      const resource = await client.readResource({ uri: 'ui://openknowledge/library.html' });
      assert.match(resource.contents[0].text, /Open Knowledge/);
      const frames = resource.contents[0]._meta.ui.csp.frameDomains;
      assert.deepEqual(frames, []);
      const bootstrap = resourceBootstrap(resource);
      assert.ok(bootstrap.viewerURL.startsWith('http://127.0.0.1:'));
      assert.equal(bootstrap.basePath, '/');
      assert.ok(!resource.contents[0].text.includes('src="/assets/openknowledge/viewer.js'));
      const tools = await client.listTools();
      assert.equal(tools.tools.length, 6);
      assert.deepEqual(tools.tools.find(tool => tool.name === 'openknowledge_library')._meta['openai/ui'].entrypoints, [{ type: 'global' }, { type: 'thread' }]);
      const result = await client.callTool({ name: 'openknowledge_library', arguments: {} });
      assert.equal(result.isError, undefined, stderr);
      assert.deepEqual(result.structuredContent.knowledgeBases.map(base => base.alias), ['packaged', 'read-only']);
      assert.equal(new URL(bootstrap.viewerURL).origin, new URL(result.structuredContent.viewerURL).origin);
      assert.equal((await fetch(result.structuredContent.panelURL)).status, 200);
      if (round === 0) {
        // Native global resources and app tools can use separate stdio pools.
        // Resource A provides a virtual route that B reads through MCP. Neither
        // process port needs browser frame or network approval.
        peer = new Client({ name: 'packaged-second-pool-test', version: '1' });
        await peer.connect(new StdioClientTransport({ command: packagedNode, args: config.args.map(value => expand(value, isolatedRoot)), env: { OPENKNOWLEDGE_REGISTRY_FILE: registry }, cwd: dir, stderr: 'pipe' }));
        const peerResource = await peer.readResource({ uri: 'ui://openknowledge/library.html' });
        const peerBootstrap = resourceBootstrap(peerResource);
        const peerHome = await peer.callTool({ name: 'openknowledge_library', arguments: {} });
        const peerOrigin = new URL(peerBootstrap.viewerURL).origin;
        assert.notEqual(peerOrigin, new URL(bootstrap.viewerURL).origin);
        assert.equal(peerOrigin, new URL(peerHome.structuredContent.viewerURL).origin);
        assert.deepEqual(peerResource.contents[0]._meta.ui.csp.frameDomains, []);
        const resourceRoute = new URL(bootstrap.viewerURL).pathname;
        const crossPool = await peer.callTool({ name: 'openknowledge_viewer_get', arguments: { route: resourceRoute } });
        assert.equal(crossPool.isError, undefined);
        assert.equal(crossPool.structuredContent.status, 200);
        assert.equal(crossPool.structuredContent.encoding, 'text');
        assert.deepEqual(resourceBootstrap(await client.readResource({ uri: 'ui://openknowledge/library.html' })), bootstrap);
        assert.equal((await fetch(bootstrap.viewerURL)).status, 200);
        assert.equal((await fetch(peerBootstrap.viewerURL)).status, 200);
        await peer.close(); peer = undefined;
        await assert.rejects(fetch(peerBootstrap.viewerURL, { signal: AbortSignal.timeout(2000) }));
        assert.equal((await fetch(bootstrap.viewerURL)).status, 200);
      }
      if (process.env.OPENKNOWLEDGE_TEST_CLI) {
        const listing = await client.callTool({ name: 'openknowledge_list_pages', arguments: { knowledgeBase: 'packaged' } });
        assert.equal(listing.isError, undefined, stderr);
        assert.ok(listing.structuredContent.pages.some(page => page.path === 'index.md'));
        const page = await client.callTool({ name: 'openknowledge_read_page', arguments: { knowledgeBase: 'packaged', path: 'index.md' } });
        assert.equal(page.isError, undefined, stderr);
        assert.match(page.structuredContent.markdown, /# Packaged note/);
        assert.equal(new URL(page.structuredContent.viewerURL).origin, new URL(bootstrap.viewerURL).origin);
        const search = await client.callTool({ name: 'openknowledge_search', arguments: { knowledgeBase: 'packaged', query: 'Packaged' } });
        assert.equal(search.isError, undefined, stderr);
        assert.ok(Array.isArray(search.structuredContent.sources));
        if (round === 0) {
          const document = await client.callTool({ name: 'openknowledge_viewer_get', arguments: { route: '/kb/packaged/api/document?path=index.md' } });
          assert.equal(document.isError, undefined, stderr);
          const current = JSON.parse(document.structuredContent.body);
          assert.equal(current.writable, true);
          const saveBody = JSON.stringify({ path: 'index.md', text: current.text + '\nSaved through native MCP.\n', expectedRevision: current.revision });
          const saved = await client.callTool({ name: 'openknowledge_viewer_save', arguments: { route: '/kb/packaged/api/document', body: saveBody } });
          assert.equal(saved.isError, undefined, stderr);
          assert.equal(saved.structuredContent.status, 200);
          assert.match(await readFile(path.join(knowledgeRoot, 'index.md'), 'utf8'), /Saved through native MCP/);
          const stale = await client.callTool({ name: 'openknowledge_viewer_save', arguments: { route: '/kb/packaged/api/document', body: saveBody } });
          assert.equal(stale.structuredContent.status, 409);
          const latest = JSON.parse(saved.structuredContent.body);
          const invalid = await client.callTool({ name: 'openknowledge_viewer_save', arguments: { route: '/kb/packaged/api/document', body: JSON.stringify({ path: 'index.md', text: '---\ntype: [unterminated\n---\n\n# Invalid YAML', expectedRevision: latest.revision }) } });
          assert.equal(invalid.structuredContent.status, 422);
          assert.equal(await readFile(path.join(knowledgeRoot, 'index.md'), 'utf8'), latest.text);
          const readonly = await client.callTool({ name: 'openknowledge_viewer_get', arguments: { route: '/kb/read-only/api/document?path=index.md' } });
          const readonlyDocument = JSON.parse(readonly.structuredContent.body);
          assert.equal(readonlyDocument.writable, false);
          const denied = await client.callTool({ name: 'openknowledge_viewer_save', arguments: { route: '/kb/read-only/api/document', body: JSON.stringify({ path: 'index.md', text: readonlyDocument.text + '\nAttempted change\n', expectedRevision: readonlyDocument.revision }) } });
          assert.equal(denied.structuredContent.status, 403);
          assert.equal(await readFile(path.join(readOnlyRoot, 'index.md'), 'utf8'), readOnlyMarkdown);
          const traversal = await client.callTool({ name: 'openknowledge_viewer_get', arguments: { route: '/kb/packaged/api/document?path=../registry.json' } });
          assert.equal(traversal.structuredContent.status, 404);
        }
      }
      // Record the SDK child's shutdown only in this test: client.close first
      // ends stdin, and any subsequent kill would indicate failed EOF cleanup.
      const child = transport._process;
      const kills = [];
      const kill = child.kill.bind(child);
      child.kill = signal => { kills.push(signal); return kill(signal); };
      const exited = new Promise(resolve => child.once('exit', (code, signal) => resolve({ code, signal })));
      await client.close(); client = undefined;
      assert.deepEqual(await exited, { code: 0, signal: null }, stderr);
      assert.deepEqual(kills, [], 'stdio EOF must close backend without an SDK kill fallback');
      await assert.rejects(fetch(result.structuredContent.viewerURL, { signal: AbortSignal.timeout(2000) }));
      await assert.rejects(fetch(result.structuredContent.panelURL, { signal: AbortSignal.timeout(2000) }));
    }
    // The packaged default finds its own CLI even without config env injection.
    const transport = new StdioClientTransport({ command: packagedNode, args: config.args.map(value => expand(value, isolatedRoot)), env: { OPENKNOWLEDGE_REGISTRY_FILE: registry }, cwd: dir, stderr: 'pipe' });
    client = new Client({ name: 'packaged-default-test', version: '1' });
    await client.connect(transport);
    const home = await client.callTool({ name: 'openknowledge_library', arguments: {} });
    assert.equal(home.isError, undefined);
    assert.equal(home.structuredContent.knowledgeBases[0].alias, 'packaged');
    await client.close(); client = undefined;
  } finally { await peer?.close(); await client?.close(); await rm(dir, { recursive: true, force: true }); }
});
