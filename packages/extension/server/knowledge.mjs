import { spawn, execFile } from 'node:child_process';
import { createHash } from 'node:crypto';
import { stat, realpath } from 'node:fs/promises';
import { homedir } from 'node:os';
import path from 'node:path';
import { promisify } from 'node:util';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';
import { startViewerPanelProxy } from './viewer-panel-proxy.mjs';

const execute = promisify(execFile);

export function normalizeViewerURL(value) {
  const url = new URL(value);
  if (url.protocol !== 'http:' || !['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname) || url.username || url.password || url.search || url.hash) {
    throw new Error('The viewer URL must be a loopback HTTP URL without credentials, query, or fragment.');
  }
  if (!url.pathname.endsWith('/')) url.pathname += '/';
  return url.href;
}

function viewerRoute(viewerURL, route, pagePath) {
  const segments = pagePath.split('/');
  if (segments.some(segment => !segment || segment === '.' || segment === '..' || segment.includes('\\'))) {
    throw new Error('Invalid page path in knowledge base.');
  }
  return new URL(route + segments.map(encodeURIComponent).join('/'), viewerURL);
}

export function pageFromResource(resource, knowledgeBase) {
  return {
    ...(knowledgeBase ? { knowledgeBase } : {}),
    path: resource.name,
    title: resource.title || resource.name.replace(/\.md$/i, '').split('/').pop(),
    description: resource.description || '',
    collection: resource.name.includes('/') ? resource.name.split('/')[0] : 'Overview',
    modified: resource.annotations?.lastModified,
  };
}

function attributedSearch(result, alias) {
  const data = result.structuredContent;
  if (!data) return result;
  const structuredContent = { ...data, knowledgeBase: alias, sources: (data.sources || []).map(source => ({ ...source, knowledgeBase: alias })) };
  return { ...result, content: [{ type: 'text', text: JSON.stringify(structuredContent) }], structuredContent };
}

// The upstream MCP inventory is the authority for paths. Never accept arbitrary
// filesystem paths or forward a caller-controlled URL to the local viewer.
export class Knowledge {
  constructor(client, viewerURL, name, metadata = {}, panel = undefined) {
    this.client = client;
    this.viewer = typeof viewerURL === 'function' ? viewerURL : () => normalizeViewerURL(viewerURL);
    this.name = name;
    this.metadata = { alias: name, name, access: 'write', managed: false, source: null, ...metadata };
    this.panel = panel;
  }

  requireAlias(alias) {
    if (alias && alias !== this.metadata.alias) throw new Error(`Unknown knowledge base: ${alias}`);
  }

  async getFrameDomains() { return [this.panel ? (await this.panel()).origin : new URL(await this.viewer()).origin]; }
  async getTestAppURL(html) {
    if (!this.panel) throw new Error('The viewer panel is unavailable.');
    return (await this.panel()).getAppURL(html);
  }

  async viewerHome() {
    const viewerURL = normalizeViewerURL(await this.viewer());
    const panel = this.panel ? await this.panel() : undefined;
    return { viewerURL, ...(panel ? { panelURL: panel.urlFor(viewerURL) } : {}) };
  }

  async getViewerBootstrap() { return this.viewerHome(); }
  async getViewerHome() { return { viewerURL: normalizeViewerURL(await this.viewer()) }; }

  async inventory() {
    const resources = [];
    let cursor;
    do {
      const result = await this.client.listResources(cursor ? { cursor } : {});
      resources.push(...result.resources);
      cursor = result.nextCursor;
    } while (cursor);
    return resources.filter(item => /\.md$/i.test(item.name));
  }

  async library(alias, selectedPath) {
    this.requireAlias(alias);
    const data = await this.list(alias);
    if (selectedPath && !data.pages.some(page => page.path === selectedPath)) throw new Error('Page is not in this knowledge base.');
    return { ...data, ...await this.viewerHome(), mode: 'root', knowledgeBases: [this.metadata], selectedKnowledgeBase: this.metadata.alias, ...(selectedPath ? { selectedPath } : {}) };
  }

  async list(alias) {
    this.requireAlias(alias);
    const resources = await this.inventory();
    return { name: this.name, knowledgeBase: this.metadata.alias, access: this.metadata.access, pages: resources.map(resource => pageFromResource(resource, this.metadata.alias)) };
  }

  async read(pagePath, alias) {
    this.requireAlias(alias);
    const resource = (await this.inventory()).find(item => item.name === pagePath);
    if (!resource) throw new Error('Page is not in this knowledge base.');
    const source = await this.client.readResource({ uri: resource.uri });
    const markdown = source.contents.filter(item => typeof item.text === 'string').map(item => item.text).join('\n');
    const viewer = normalizeViewerURL(await this.viewer());
    const url = viewerRoute(viewer, 'api/file/', pagePath);
    const response = await fetch(url, { signal: AbortSignal.timeout(15000) });
    if (!response.ok) throw new Error(`The Open Knowledge renderer returned ${response.status}.`);
    const rendered = await response.json();
    const viewerURL = viewerRoute(viewer, 'file/', pagePath);
    const editorURL = new URL(viewerURL);
    editorURL.searchParams.set('edit', '1');
    const writable = this.metadata.access === 'write' && !this.metadata.managed;
    const panel = this.panel ? await this.panel() : undefined;
    return {
      ...pageFromResource(resource, this.metadata.alias),
      title: resource.title || rendered.title || pagePath,
      html: rendered.body,
      markdown,
      access: this.metadata.access,
      sourceURI: resource.uri,
      revision: createHash('sha256').update(markdown).digest('hex'),
      viewerURL: viewerURL.href,
      ...(writable ? { editorURL: editorURL.href } : {}),
      ...(panel ? { panelURL: panel.urlFor(viewerURL.href), ...(writable ? { panelEditorURL: panel.urlFor(editorURL.href) } : {}) } : {}),
    };
  }

  async search(query, alias) {
    this.requireAlias(alias);
    return attributedSearch(await this.client.callTool({ name: 'openknowledge_search', arguments: { query, limit: 12, budget: 4000 } }), this.metadata.alias);
  }
}

export function registryMetadata(entry) {
  const managed = Boolean(entry.managed || entry.source?.type || entry.source?.url || entry.source?.managedRoot);
  return { alias: entry.name, name: entry.name, access: entry.access === 'write' && !managed ? 'write' : 'read', managed, source: entry.source || null };
}

// The original viewer owns navigation and workspace UI. One viewer starts on
// demand for the library. MCP processes start only for a scoped inventory/read
// or an explicitly requested cross-library search. Reconcile changed leases.
export class RegistryKnowledge {
  constructor({ loadRegistry, openBase, startViewer }) {
    this.loadRegistry = loadRegistry;
    this.openBase = openBase;
    this.startViewer = startViewer;
    this.bases = new Map();
    this.retired = new Set();
    this.closed = false;
  }

  requireOpen() { if (this.closed) throw new Error('The Open Knowledge library is closed.'); }

  closeBase(cached) {
    if (!cached.closing) cached.closing = cached.backend.then(backend => backend.close(), () => {});
    return cached.closing;
  }

  async registry() {
    this.requireOpen();
    if (!this.loading) {
      this.loading = Promise.resolve().then(() => this.loadRegistry()).then(async entries => {
        if (!Array.isArray(entries)) throw new Error('Invalid Open Knowledge registry inventory.');
        const names = new Set();
        for (const entry of entries) {
          if (!entry || typeof entry.name !== 'string' || !entry.name || names.has(entry.name)) throw new Error('Invalid or duplicate registry alias.');
          names.add(entry.name);
        }
        const closing = [];
        for (const [alias, cached] of this.bases) {
          const entry = entries.find(item => item.name === alias);
          if (!entry || JSON.stringify(entry) !== cached.identity) {
            this.bases.delete(alias);
            const retired = this.closeBase(cached);
            this.retired.add(retired);
            void retired.then(() => this.retired.delete(retired), () => this.retired.delete(retired));
            closing.push(retired);
          }
        }
        await Promise.all(closing);
        this.requireOpen();
        return entries;
      }).finally(() => { this.loading = undefined; });
    }
    return this.loading;
  }

  entry(entries, alias) {
    if (!alias) throw new Error('Select a knowledge base from the library first.');
    const entry = entries.find(item => item.name === alias);
    if (!entry) throw new Error(`Unknown knowledge base: ${alias}`);
    return entry;
  }

  async base(entry) {
    this.requireOpen();
    let cached = this.bases.get(entry.name);
    if (!cached) {
      cached = { identity: JSON.stringify(entry) };
      cached.backend = Promise.resolve().then(() => this.openBase(entry, async () => {
        const viewer = await this.viewerBackend();
        return new URL(`kb/${encodeURIComponent(entry.name)}/`, viewer.url).href;
      }, async () => (await this.viewerBackend()).getPanel())).catch(error => {
        if (this.bases.get(entry.name) === cached) this.bases.delete(entry.name);
        throw error;
      });
      this.bases.set(entry.name, cached);
    }
    const backend = await cached.backend;
    this.requireOpen();
    return backend.knowledge;
  }

  async viewerBackend() {
    this.requireOpen();
    if (!this.viewer) {
      this.viewer = Promise.resolve().then(() => this.startViewer()).catch(error => { this.viewer = undefined; throw error; });
    }
    const backend = await this.viewer;
    this.requireOpen();
    return backend;
  }

  async getFrameDomains() {
    const viewer = await this.viewerBackend();
    return [viewer.getPanel ? (await viewer.getPanel()).origin : new URL(viewer.url).origin];
  }

  async getTestAppURL(html) { return (await (await this.viewerBackend()).getPanel()).getAppURL(html); }

  async viewerHome(alias) {
    const viewer = await this.viewerBackend();
    const root = normalizeViewerURL(viewer.url);
    const viewerURL = alias ? new URL(`kb/${encodeURIComponent(alias)}/`, root).href : root;
    const panel = viewer.getPanel ? await viewer.getPanel() : undefined;
    return { viewerURL, ...(panel ? { panelURL: panel.urlFor(viewerURL) } : {}) };
  }

  async getViewerBootstrap() { return this.viewerHome(); }
  async getViewerHome() { return { viewerURL: normalizeViewerURL((await this.viewerBackend()).url) }; }

  async library(alias, selectedPath) {
    const entries = await this.registry();
    const home = { mode: 'registry', name: 'Open Knowledge', knowledgeBases: entries.map(registryMetadata), pages: [] };
    if (!alias) {
      if (selectedPath) throw new Error('A knowledge base is required for a selected page.');
      return { ...home, ...await this.viewerHome() };
    }
    const knowledge = await this.base(this.entry(entries, alias));
    const data = await knowledge.list();
    if (selectedPath && !data.pages.some(page => page.path === selectedPath)) throw new Error('Page is not in this knowledge base.');
    return { ...home, ...data, ...await this.viewerHome(alias), selectedKnowledgeBase: alias, ...(selectedPath ? { selectedPath } : {}) };
  }

  async list(alias) { return (await this.base(this.entry(await this.registry(), alias))).list(); }
  async read(pagePath, alias) { return (await this.base(this.entry(await this.registry(), alias))).read(pagePath); }

  async search(query, alias) {
    const entries = await this.registry();
    if (alias) return (await this.base(this.entry(entries, alias))).search(query);
    const results = [];
    let next = 0;
    // Bound process startup pressure while searching the complete registry.
    await Promise.all(Array.from({ length: Math.min(3, entries.length) }, async () => {
      for (let index; (index = next++) < entries.length;) {
        const entry = entries[index];
        try {
          const result = await (await this.base(entry)).search(query);
          if (result.isError) throw new Error(result.content?.find(item => item.type === 'text')?.text || 'Search failed.');
          results[index] = { knowledgeBase: entry.name, result };
        } catch (error) { results[index] = { knowledgeBase: entry.name, error: error.message }; }
      }
    }));
    const structuredContent = {
      query,
      sources: results.flatMap(item => (item.result?.structuredContent?.sources || []).map(source => ({ ...source, knowledgeBase: item.knowledgeBase }))),
      errors: results.filter(item => item.error).map(({ knowledgeBase, error }) => ({ knowledgeBase, error })),
    };
    return { content: [{ type: 'text', text: JSON.stringify(structuredContent) }], structuredContent };
  }

  async close() {
    if (this.closing) return this.closing;
    this.closed = true;
    this.closing = Promise.allSettled([
      ...[...this.bases.values()].map(cached => this.closeBase(cached)),
      ...this.retired,
      ...(this.viewer ? [this.viewer.then(backend => backend.close())] : []),
    ]).then(() => { this.bases.clear(); });
    return this.closing;
  }
}

function cli(config) {
  return { command: config.binary || 'go', prefix: config.binary ? [] : ['run', './packages/cli/cmd/openknowledge'], cwd: config.repoRoot, env: { ...process.env, OPENKNOWLEDGE_TELEMETRY_SUPPRESS: '1' } };
}

export async function loadRegistry(config) {
  const { command, prefix, cwd, env } = cli(config);
  const { stdout } = await execute(command, [...prefix, 'registry', 'list', '--json'], { cwd, env, timeout: 90000, maxBuffer: 4 * 1024 * 1024 });
  const report = JSON.parse(stdout);
  if (!Array.isArray(report.entries)) throw new Error('Invalid Open Knowledge registry inventory.');
  return report.entries;
}

async function startViewer(config) {
  const withPanel = (url, close) => {
    let panel;
    return {
      url,
      getPanel() {
        if (!panel) panel = startViewerPanelProxy({ viewerURL: new URL('/', url).href }).catch(error => { panel = undefined; throw error; });
        return panel;
      },
      async close() { await Promise.all([close(), ...(panel ? [panel.then(proxy => proxy.close())] : [])]); },
    };
  };
  if (config.viewerURL) return withPanel(normalizeViewerURL(config.viewerURL), async () => {});
  const { command, prefix, cwd, env } = cli(config);
  const args = [...prefix, 'view', '--no-browser', '--host', '127.0.0.1', '--port', '0', ...(config.root ? [config.root] : [])];
  const process = spawn(command, args, { cwd, env, detached: globalThis.process.platform !== 'win32', stdio: ['ignore', 'pipe', 'pipe'] });
  const stop = () => {
    if (process.exitCode !== null) return;
    if (globalThis.process.platform !== 'win32' && process.pid) {
      try { globalThis.process.kill(-process.pid, 'SIGTERM'); return; } catch {}
    }
    process.kill('SIGTERM');
  };
  process.stderr.on('data', chunk => globalThis.process.stderr.write(chunk));
  try {
    const url = await new Promise((resolve, reject) => {
      let output = '';
      let settled = false;
      const timer = setTimeout(() => finish(new Error('Viewer startup timed out after 90 seconds.')), 90000);
      const finish = (error, value) => {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        process.off('error', fail);
        process.off('exit', exited);
        error ? reject(error) : resolve(value);
      };
      const fail = error => finish(error);
      const exited = code => finish(new Error(`Viewer exited during startup (${code}).`));
      process.once('error', fail);
      process.once('exit', exited);
      process.stdout.on('data', chunk => {
        output = (output + chunk.toString()).slice(-32768);
        const match = output.match(/Open Knowledge view: (https?:\/\/\S+)/);
        // A one-entry registry prints its alias as a convenience URL. Registry
        // routes still start at the origin, unlike explicit single-root mode.
        if (match) finish(null, normalizeViewerURL(config.root ? match[1] : new URL('/', match[1]).href));
      });
    });
    return withPanel(url, async () => stop());
  } catch (error) { stop(); throw error; }
}

async function openBase(config, entry, viewerURL, panel) {
  // Only registry-owned roots reach the subprocess. A missing base remains in
  // the library inventory and produces an actionable error when selected.
  const root = entry.path?.startsWith('~/') ? path.join(homedir(), entry.path.slice(2)) : entry.path;
  if (!root) throw new Error(`Knowledge base ${entry.name} has no local root.`);
  let info;
  try { info = await stat(root); } catch { throw new Error(`Knowledge base ${entry.name} is unavailable: its local root does not exist.`); }
  if (!info.isDirectory()) throw new Error(`Knowledge base ${entry.name} is unavailable: its local root is not a directory.`);
  const { command, prefix, cwd, env } = cli(config);
  const client = new Client({ name: 'openknowledge-extension', version: '0.0.1' });
  try {
    await client.connect(new StdioClientTransport({ command, args: [...prefix, 'mcp', root], cwd, env, stderr: 'inherit' }));
    return { knowledge: new Knowledge(client, viewerURL, entry.name, registryMetadata(entry), panel), async close() { await client.close(); } };
  } catch (error) { await client.close(); throw error; }
}

export async function startKnowledge(config) {
  if (!config.root) {
    const knowledge = new RegistryKnowledge({ loadRegistry: () => loadRegistry(config), openBase: (entry, viewerURL, panel) => openBase(config, entry, viewerURL, panel), startViewer: () => startViewer(config) });
    return { knowledge, close: () => knowledge.close() };
  }
  const viewer = await startViewer(config);
  try {
    const aliasSegments = new URL(viewer.url).pathname.split('/').filter(Boolean);
    const alias = !config.viewerURL && aliasSegments.length === 1 ? decodeURIComponent(aliasSegments[0]) : path.basename(config.root);
    const entry = { name: alias, path: config.root, access: 'write' };
    const canonicalRoot = await realpath(config.root);
    const candidates = await Promise.all((await loadRegistry(config)).map(async item => {
      const root = item.path?.startsWith('~/') ? path.join(homedir(), item.path.slice(2)) : item.path;
      if (!root) return undefined;
      try {
        const canonical = await realpath(root);
        return canonicalRoot === canonical || canonicalRoot.startsWith(canonical + path.sep) ? { item, canonical } : undefined;
      } catch { return undefined; }
    }));
    const match = candidates.filter(Boolean).sort((a, b) => b.canonical.length - a.canonical.length)[0];
    const registered = match?.item;
    if (registered) {
      entry.access = registryMetadata(registered).access;
      if (config.viewerURL && canonicalRoot === match.canonical) entry.name = registered.name;
    }
    const backend = await openBase(config, entry, viewer.url, () => viewer.getPanel());
    let closed = false;
    return { knowledge: backend.knowledge, async close() { if (closed) return; closed = true; await Promise.all([viewer.close(), backend.close()]); } };
  } catch (error) { await viewer.close(); throw error; }
}
