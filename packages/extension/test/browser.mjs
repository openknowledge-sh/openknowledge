import { test } from 'node:test';
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { mkdtemp, mkdir, writeFile, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'node:net';
import { chromium } from 'playwright';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..');
const markdown = (title, body) => `---\ntype: Guide\ntitle: ${title}\n---\n\n# ${title}\n\n${body}\n`;
async function freePort() {
  const server = createServer();
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const port = server.address().port;
  await new Promise(resolve => server.close(resolve));
  return port;
}
async function openDocument(viewer, base, pagePath) {
  const sidebar = viewer.locator('[data-file-sidebar]');
  if (await sidebar.getAttribute('aria-hidden') === 'true') await viewer.locator('[data-sidebar-toggle]').first().click();
  const group = viewer.locator(`.knowledge-base-group[data-knowledge-base="${base}"]`);
  const disclosure = group.locator('[data-knowledge-base-disclosure]');
  if (await disclosure.getAttribute('aria-expanded') === 'false') await disclosure.click();
  const directories = pagePath.split('/'); directories.pop();
  let prefix = '';
  for (const name of directories) {
    prefix = prefix ? `${prefix}/${name}` : name;
    const directory = group.locator(`[data-tree-directory-path="${prefix}"]`);
    if (await directory.getAttribute('aria-expanded') === 'false') await directory.click();
  }
  await group.locator(`a[data-tree-path="${pagePath}"]`).click();
}

test('whole viewer: workspaces, native tree/search, editor, context and isolated MCP host', { timeout: 180000 }, async () => {
  const temporary = await mkdtemp(path.join(tmpdir(), 'okn-extension-browser-'));
  const alpha = path.join(temporary, 'alpha');
  const beta = path.join(temporary, 'beta');
  let browser, server;
  let output = '';
  try {
    await mkdir(path.join(alpha, 'guides'), { recursive: true });
    await mkdir(beta);
    await mkdir(path.join(alpha, 'media'));
    await writeFile(path.join(alpha, 'index.md'), markdown('Alpha Home', 'Library context from alpha.'));
    await writeFile(path.join(alpha, 'guides/note.md'), markdown('Alpha Guide', 'Original alpha passage.\n\n![Viewer image](../media/pixel.png)'));
    await writeFile(path.join(beta, 'index.md'), markdown('Beta Home', 'Library context from beta.'));
    await writeFile(path.join(alpha, 'media/pixel.png'), Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGPQSPb9DwADZwHYO30dDAAAAABJRU5ErkJggg==', 'base64'));
    const registryFile = path.join(temporary, 'registry.json');
    await writeFile(registryFile, JSON.stringify({ schemaVersion: '1', connections: {
      [alpha]: { key: 'alpha', access: 'write' }, [beta]: { key: 'beta', access: 'read' },
    } }));
    const binary = process.env.OPENKNOWLEDGE_BIN || path.join(temporary, 'okn');
    if (!process.env.OPENKNOWLEDGE_BIN) execFileSync('go', ['build', '-o', binary, './packages/cli/cmd/openknowledge'], { cwd: root, env: { ...process.env, GOCACHE: path.join(temporary, 'go-cache') }, timeout: 90000 });
    const port = await freePort();
    server = spawn(process.execPath, ['packages/extension/scripts/dev.mjs', '--port', String(port)], { cwd: root, env: { ...process.env, OPENKNOWLEDGE_BIN: binary, OPENKNOWLEDGE_REGISTRY_FILE: registryFile }, stdio: ['ignore', 'pipe', 'pipe'] });
    server.stdout.on('data', chunk => { output += chunk; });
    server.stderr.on('data', chunk => { output += chunk; });
    await new Promise((resolve, reject) => {
      const timeout = setTimeout(() => reject(new Error(`Extension startup timed out: ${output}`)), 30000);
      const check = chunk => { if (String(chunk).includes('extension preview:')) { clearTimeout(timeout); resolve(); } };
      server.stdout.on('data', check);
      server.once('exit', code => { clearTimeout(timeout); reject(new Error(`Extension exited ${code}: ${output}`)); });
    });
    browser = await chromium.launch({ headless: true });
    const page = await browser.newPage({ viewport: { width: 1400, height: 950 } });
    const errors = [];
    const resourceErrors = [];
    page.on('pageerror', error => errors.push(error.message));
    page.on('response', response => {
      if (response.status() >= 400) resourceErrors.push({ status: response.status(), path: new URL(response.url()).pathname.replace(/\/[a-f0-9]{64}/g, '/[proxy]') });
    });
    await page.goto(`http://127.0.0.1:${port}/`);
    const viewer = page.frameLocator('#viewer-container iframe');
    await page.locator('#viewer-status').filter({ hasText: 'ready' }).waitFor();
    assert.equal(await page.locator('.navigation, .base-card, #reader').count(), 0);
    assert.equal(await viewer.locator('.knowledge-base-group').count(), 2);
    await viewer.locator('[data-file-sidebar][aria-hidden="false"]').waitFor();
    await viewer.locator('[data-graph-view-toggle]').click();
    await viewer.locator('html[data-viewer-view="graph"]').waitFor({ state: 'attached' });
    await page.locator('#context-button[disabled]').waitFor({ state: 'attached' });
    await viewer.locator('[data-documents-view-toggle]').click();
    await openDocument(viewer, 'alpha', 'guides/note.md');
    const editor = viewer.locator('.is-active-panel .document-rich-content');
    await editor.waitFor();
    await page.locator('#active-page').filter({ hasText: 'alpha / guides/note.md' }).waitFor();
    const image = editor.locator('img');
    await image.waitFor();
    assert.equal(await image.evaluate(element => new Promise(resolve => {
      if (element.complete) resolve(element.naturalWidth > 0);
      else { element.onload = () => resolve(element.naturalWidth > 0); element.onerror = () => resolve(false); }
    })), true, `Actual viewer images must render through the protected proxy: ${JSON.stringify(resourceErrors)}`);
    const originalFrame = await page.locator('#viewer-container iframe').getAttribute('src');
    await editor.click();
    await page.keyboard.press('ControlOrMeta+End');
    await page.keyboard.type(' Unsaved extension draft.');
    await viewer.locator('.is-active-panel .document-editor-status').filter({ hasText: 'Not saved' }).waitFor();
    const draftBeforeUndo = await editor.innerText();
    await viewer.getByRole('button', { name: 'Undo', exact: true }).click();
    assert.notEqual(await editor.innerText(), draftBeforeUndo);
    await viewer.getByRole('button', { name: 'Redo', exact: true }).click();
    assert.equal(await editor.innerText(), draftBeforeUndo);
    await openDocument(viewer, 'alpha', 'index.md');
    await editor.filter({ hasText: 'Alpha Home' }).waitFor();
    await openDocument(viewer, 'alpha', 'guides/note.md');
    await editor.filter({ hasText: 'Unsaved extension draft' }).waitFor();
    assert.equal(await editor.innerText(), draftBeforeUndo);
    assert.equal(await page.locator('#viewer-container iframe').getAttribute('src'), originalFrame);
    assert.ok(!(await readFile(path.join(alpha, 'guides/note.md'), 'utf8')).includes('Unsaved extension draft'));
    await editor.evaluate(element => {
      const range = document.createRange(); range.selectNodeContents(element);
      const selection = window.getSelection(); selection.removeAllRanges(); selection.addRange(range);
    });
    await page.locator('#context-button').filter({ hasText: 'Use selection' }).waitFor();
    await page.locator('#context-button').click();
    await page.locator('#context-output').filter({ hasText: 'Unsaved extension draft' }).waitFor();
    const draftContext = JSON.parse(await page.locator('#context-output').textContent());
    assert.equal(draftContext.dirty, true);
    assert.equal(draftContext.knowledgeBase, 'alpha');
    assert.equal(draftContext.revision, undefined);
    await viewer.getByRole('button', { name: 'Save', exact: true }).click();
    await viewer.locator('.is-active-panel .document-editor-status').filter({ hasText: /^Saved/ }).waitFor();
    const savedMarkdown = await readFile(path.join(alpha, 'guides/note.md'), 'utf8');
    assert.match(savedMarkdown, /Unsaved extension draft/);
    assert.ok(savedMarkdown.includes('../media/pixel.png'), 'Viewer integration must preserve source image paths when saving');
    await editor.evaluate(() => window.getSelection().removeAllRanges());
    await page.locator('#context-button').filter({ hasText: 'Preview context' }).waitFor();
    await page.locator('#context-button').click();
    const context = JSON.parse(await page.locator('#context-output').textContent());
    assert.equal(context.knowledgeBase, 'alpha');
    assert.equal(context.path, 'guides/note.md');
    assert.match(context.text, /Unsaved extension draft/);
    await page.locator('#context-preview summary').click();
    await viewer.locator('[data-primary-search] .search-input').fill('Original alpha passage');
    const result = viewer.locator('.search-result[data-knowledge-base="alpha"]').filter({ hasText: 'Original alpha passage' });
    await result.waitFor();
    await result.click();
    await editor.filter({ hasText: 'Alpha Guide' }).waitFor();
    await openDocument(viewer, 'beta', 'index.md');
    await page.locator('#active-page').filter({ hasText: 'beta / index.md' }).waitFor();
    await viewer.locator('.is-active-panel .note-body').filter({ hasText: 'Library context from beta' }).waitFor();
    assert.equal(await viewer.locator('.is-active-panel .document-rich-content').count(), 0);
    await page.goto(`http://127.0.0.1:${port}/host.html`);
    await page.locator('#status').filter({ hasText: /MCP Apps bridge connected|Context received/ }).waitFor();
    const app = page.frameLocator('#mcp-app');
    const nativeViewer = app.frameLocator('#viewer-container iframe');
    await app.locator('#viewer-status').filter({ hasText: 'ready' }).waitFor();
    await openDocument(nativeViewer, 'alpha', 'guides/note.md');
    await nativeViewer.locator('.is-active-panel .document-rich-content').filter({ hasText: 'Alpha Guide' }).waitFor();
    await app.locator('#context-button').click();
    await page.locator('#context').filter({ hasText: 'guides/note.md' }).waitFor({ state: 'attached' });
    assert.match(await page.locator('#context').textContent(), /alpha/);
    await openDocument(nativeViewer, 'alpha', 'index.md');
    await app.locator('#active-page').filter({ hasText: 'alpha / index.md' }).waitFor();
    await page.locator('#context').filter({ hasText: 'The user is viewing' }).waitFor({ state: 'attached' });
    assert.ok(JSON.parse(await page.locator('#context').textContent()).content[0].text.includes('path="index.md"'));
    await openDocument(nativeViewer, 'alpha', 'guides/note.md');
    await nativeViewer.locator('.is-active-panel .document-rich-content').filter({ hasText: 'Alpha Guide' }).waitFor();
    for (const width of [1400, 560]) {
      await page.setViewportSize({ width, height: 950 });
      await nativeViewer.locator('.is-active-panel .document-rich-content').evaluate(element => new Promise(resolve => requestAnimationFrame(() => resolve(element.offsetWidth))));
      const bounds = await nativeViewer.locator('.is-active-panel .document-rich-content').evaluate(element => {
        const content = element.getBoundingClientRect();
        const save = [...document.querySelectorAll('.is-active-panel button')].find(button => button.textContent.trim() === 'Save').getBoundingClientRect();
        return { width: innerWidth, content: { left: content.left, right: content.right }, save: { left: save.left, right: save.right } };
      });
      assert.ok(bounds.content.left >= 0 && bounds.content.right <= bounds.width + 1, `Document must fit at ${width}px: ${JSON.stringify(bounds)}`);
      assert.ok(bounds.save.left >= 0 && bounds.save.right <= bounds.width + 1, `Save must fit at ${width}px: ${JSON.stringify(bounds)}`);
      if (width === 560) {
        await nativeViewer.locator('[data-sidebar-toggle]').first().click();
        await nativeViewer.locator('[data-file-sidebar][aria-hidden="false"]').waitFor();
        await nativeViewer.locator('[data-sidebar-toggle]').first().click();
        await page.screenshot({ path: '/private/tmp/openknowledge-extension-narrow.png' });
      }
    }
    await page.setViewportSize({ width: 1400, height: 950 });
    await nativeViewer.locator('body').evaluate(element => Promise.all(element.getAnimations({ subtree: true })
      .filter(animation => animation.effect?.getComputedTiming().iterations !== Infinity)
      .map(animation => animation.finished.catch(() => {}))));
    const hostURL = new URL(page.url());
    const appURL = new URL(await page.locator('#mcp-app').getAttribute('src'));
    assert.notEqual(appURL.origin, hostURL.origin);
    assert.equal(await page.locator('#mcp-app').getAttribute('srcdoc'), null);
    assert.deepEqual(errors, []);
    await page.screenshot({ path: '/private/tmp/openknowledge-extension-panel.png' });
    // Keep the browser's actual CSP while omitting its optional bridge report.
    // Tool URLs also come from another simulated MCP pool and must not become
    // iframe or external destinations in this resource.
    await page.goto(`http://127.0.0.1:${port}/host.html?compatibility=omitted-csp-split-tools`);
    await app.locator('#viewer-status').filter({ hasText: 'ready' }).waitFor();
    const declaredApp = new URL(await page.locator('#mcp-app').getAttribute('src'));
    assert.equal(new URL(await app.locator('#viewer-container iframe').getAttribute('src')).origin, declaredApp.origin);
    await openDocument(nativeViewer, 'alpha', 'guides/note.md');
    await app.locator('#active-page').filter({ hasText: 'alpha / guides/note.md' }).waitFor();
    await app.locator('#context-button').click();
    await page.locator('#context').filter({ hasText: 'guides/note.md' }).waitFor({ state: 'attached' });
    assert.equal(new URL(await app.locator('#viewer-container iframe').getAttribute('src')).origin, declaredApp.origin);
    assert.equal(await app.locator('#fallback').isVisible(), false);
    // Missing reports must still respect a real host CSP denial.
    await page.route('**/_mcp-app', async route => {
      const response = await route.fetch();
      const headers = response.headers();
      headers['content-security-policy'] = headers['content-security-policy'].replace(/frame-src [^;]+/, "frame-src 'none'");
      await route.fulfill({ response, headers });
    });
    await page.goto(`http://127.0.0.1:${port}/host.html?compatibility=omitted-csp-split-tools`);
    await app.locator('#fallback-message').filter({ hasText: 'host blocked' }).waitFor();
    assert.equal(await app.locator('#viewer-container iframe').count(), 0);
    assert.deepEqual(errors, []);
    console.log('Browser flow passed: whole viewer/editor/context, omitted CSP report, split tool addresses, and enforced browser CSP denial.');
  } finally {
    await browser?.close();
    if (server && server.exitCode === null) {
      const exited = new Promise(resolve => server.once('exit', resolve));
      server.kill('SIGTERM');
      await Promise.race([exited, new Promise(resolve => setTimeout(resolve, 5000))]);
      if (server.exitCode === null) server.kill('SIGKILL');
    }
    await rm(temporary, { recursive: true, force: true });
  }
});
