import { test } from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
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

test('native resource reuses the entire viewer without loopback frames or browser connections', { timeout: 180000 }, async () => {
  const temporary = await mkdtemp(path.join(tmpdir(), 'okn-extension-native-browser-'));
  let browser, server;
  let output = '';
  try {
    const alpha = path.join(temporary, 'alpha');
    const beta = path.join(temporary, 'beta');
    await mkdir(path.join(alpha, 'guides'), { recursive: true });
    await mkdir(path.join(alpha, 'media'));
    await mkdir(beta);
    await writeFile(path.join(alpha, 'index.md'), markdown('Alpha Home', 'Home source material.'));
    await writeFile(path.join(alpha, 'guides/note.md'), markdown('Alpha Guide', 'Original alpha passage.\n\n![Viewer image](../media/pixel.png)'));
    await writeFile(path.join(alpha, 'media/pixel.png'), Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGPQSPb9DwADZwHYO30dDAAAAABJRU5ErkJggg==', 'base64'));
    await writeFile(path.join(beta, 'index.md'), markdown('Beta Home', 'Read-only beta document.'));
    const registryFile = path.join(temporary, 'registry.json');
    await writeFile(registryFile, JSON.stringify({ schemaVersion: '1', connections: {
      [alpha]: { key: 'alpha', access: 'write' }, [beta]: { key: 'beta', access: 'read' },
    } }));
    assert.ok(process.env.OPENKNOWLEDGE_BIN, 'Set OPENKNOWLEDGE_BIN to the built Open Knowledge CLI.');
    const port = await freePort();
    server = spawn(process.execPath, ['packages/extension/scripts/dev.mjs', '--port', String(port)], { cwd: root, env: { ...process.env, OPENKNOWLEDGE_REGISTRY_FILE: registryFile }, stdio: ['ignore', 'pipe', 'pipe'] });
    server.stdout.on('data', chunk => { output += chunk; });
    server.stderr.on('data', chunk => { output += chunk; });
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error(`Native test startup timed out: ${output}`)), 30000);
      server.stdout.on('data', chunk => { if (String(chunk).includes('extension preview:')) { clearTimeout(timer); resolve(); } });
      server.once('exit', code => { clearTimeout(timer); reject(new Error(`Native test server exited ${code}: ${output}`)); });
    });
    browser = await chromium.launch({ headless: true });
    const page = await browser.newPage({ viewport: { width: 1400, height: 950 } });
    const errors = [];
    const appRequests = [];
    page.on('pageerror', error => errors.push(error.message));
    page.on('request', request => {
      if (request.frame().parentFrame() && !request.isNavigationRequest() && /^https?:/.test(request.url())) appRequests.push(request.url());
    });
    // Match the native restriction that caused the user's screenshot. The UI
    // may communicate with its parent through MCP Apps, but cannot fetch HTTP
    // or render another local viewer frame.
    await page.route('**/_mcp-app', async route => {
      const response = await route.fetch();
      const headers = response.headers();
      headers['content-security-policy'] = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; media-src data: blob:; connect-src 'none'; frame-src 'none'; base-uri 'none'; form-action 'none'";
      await route.fulfill({ response, headers });
    });
    await page.goto(`http://127.0.0.1:${port}/host.html?mode=native`);
    const viewer = page.frameLocator('#mcp-app');
    await viewer.locator('#okn-native-status').filter({ hasText: /ready/i }).waitFor();
    assert.equal(await viewer.locator('iframe').count(), 0);
    assert.equal(await viewer.locator('.knowledge-base-group').count(), 2);
    await openDocument(viewer, 'alpha', 'guides/note.md');
    const editor = viewer.locator('.is-active-panel .document-rich-content');
    await editor.filter({ hasText: 'Alpha Guide' }).waitFor();
    await viewer.locator('#okn-native-active-page').filter({ hasText: 'alpha / guides/note.md' }).waitFor();
    const image = editor.locator('img');
    await image.waitFor();
    await image.evaluate(element => new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('Native image did not render')), 10000);
      const check = () => { if (element.src.startsWith('data:image/png;base64,') && element.complete && element.naturalWidth > 0) { clearTimeout(timer); resolve(); } else setTimeout(check, 20); };
      check();
    }));
    await editor.click();
    await page.keyboard.press('ControlOrMeta+End');
    await page.keyboard.type(' Native extension draft.');
    await viewer.locator('.is-active-panel .document-editor-status').filter({ hasText: 'Not saved' }).waitFor();
    const draft = await editor.innerText();
    await viewer.getByRole('button', { name: 'Undo', exact: true }).click();
    assert.notEqual(await editor.innerText(), draft);
    await viewer.getByRole('button', { name: 'Redo', exact: true }).click();
    assert.equal(await editor.innerText(), draft);
    await openDocument(viewer, 'alpha', 'index.md');
    await editor.filter({ hasText: 'Alpha Home' }).waitFor();
    await openDocument(viewer, 'alpha', 'guides/note.md');
    await editor.filter({ hasText: 'Native extension draft' }).waitFor();
    assert.equal(await editor.innerText(), draft);
    assert.doesNotMatch(await readFile(path.join(alpha, 'guides/note.md'), 'utf8'), /Native extension draft/);
    await editor.evaluate(element => {
      const range = document.createRange(); range.selectNodeContents(element);
      const selection = window.getSelection(); selection.removeAllRanges(); selection.addRange(range);
    });
    await viewer.locator('#okn-native-context-button').filter({ hasText: /selection/i }).waitFor();
    await viewer.locator('#okn-native-context-button').click();
    await page.locator('#context').filter({ hasText: 'Native extension draft' }).waitFor({ state: 'attached' });
    const selectionContext = JSON.parse(JSON.parse(await page.locator('#context').textContent()).content[0].text.split('\n').slice(1).join('\n'));
    assert.equal(selectionContext.dirty, true);
    assert.equal(selectionContext.knowledgeBase, 'alpha');
    await viewer.getByRole('button', { name: 'Save', exact: true }).click();
    await viewer.locator('.is-active-panel .document-editor-status').filter({ hasText: /^Saved/ }).waitFor();
    const saved = await readFile(path.join(alpha, 'guides/note.md'), 'utf8');
    assert.match(saved, /Native extension draft/);
    assert.match(saved, /\.\.\/media\/pixel\.png/, 'The native adapter must preserve source image paths on save.');
    await editor.evaluate(() => window.getSelection().removeAllRanges());
    await viewer.locator('#okn-native-context-button').filter({ hasText: /saved page/i }).waitFor();
    await viewer.locator('#okn-native-context-button').click();
    await page.locator('#context').filter({ hasText: 'revision' }).waitFor({ state: 'attached' });
    assert.match(await page.locator('#context').textContent(), /Native extension draft/);
    await viewer.locator('[data-primary-search] .search-input').fill('Original alpha passage');
    const result = viewer.locator('.search-result[data-knowledge-base="alpha"]').filter({ hasText: 'Original alpha passage' });
    await result.waitFor(); await result.click();
    await editor.filter({ hasText: 'Alpha Guide' }).waitFor();
    await openDocument(viewer, 'beta', 'index.md');
    await viewer.locator('.is-active-panel .note-body').filter({ hasText: 'Read-only beta document' }).waitFor();
    assert.equal(await viewer.locator('.is-active-panel .document-rich-content').count(), 0);
    await viewer.locator('[data-graph-view-toggle]').click();
    await viewer.locator('html[data-viewer-view="graph"]').waitFor({ state: 'attached' });
    await viewer.locator('#okn-native-context-button[disabled]').waitFor({ state: 'attached' });
    await viewer.locator('[data-documents-view-toggle]').click();
    await openDocument(viewer, 'alpha', 'guides/note.md');
    await editor.filter({ hasText: 'Native extension draft' }).waitFor();
    assert.deepEqual(errors, []);
    assert.deepEqual(appRequests, [], 'Native resource must not use browser HTTP for viewer assets or APIs.');
    await viewer.locator('body').evaluate(element => Promise.all(element.ownerDocument.getAnimations()
      .filter(animation => animation.effect?.getComputedTiming().iterations !== Infinity)
      .map(animation => animation.finished.catch(() => {}))));
    const controls = await viewer.locator('.okn-native-context-bar').evaluate(element => {
      const bounds = element.getBoundingClientRect();
      return { left: bounds.left, right: bounds.right, width: innerWidth, opacity: getComputedStyle(element).opacity };
    });
    assert.ok(controls.left >= 0 && controls.right <= controls.width, `Context controls must fit: ${JSON.stringify(controls)}`);
    assert.equal(controls.opacity, '1');
    await viewer.locator('[data-viewer-settings-trigger]').click();
    await viewer.locator('[data-viewer-settings-menu]').waitFor();
    await viewer.locator('[data-viewer-settings-trigger]').click();
    await page.screenshot({ path: '/private/tmp/openknowledge-extension-native-panel.png' });
    for (const width of [560, 360]) {
      await page.setViewportSize({ width, height: 950 });
      const narrow = await viewer.locator('.okn-native-context-bar').evaluate(element => {
        const bounds = element.getBoundingClientRect();
        return { left: bounds.left, right: bounds.right, width: innerWidth,
          buttons: [...element.querySelectorAll('button')].map(button => { const box = button.getBoundingClientRect(); return { left: box.left, right: box.right }; }) };
      });
      assert.ok(narrow.left >= 0 && narrow.right <= narrow.width && narrow.buttons.every(button => button.left >= 0 && button.right <= narrow.width), `Native context controls must fit at ${width}px: ${JSON.stringify(narrow)}`);
    }
    await page.setViewportSize({ width: 1400, height: 950 });
    // A host is also free to give its MCP Apps resource an opaque origin. Its
    // URL/history/storage must still operate through the virtual viewer scope.
    await page.goto(`http://127.0.0.1:${port}/host.html?mode=native-opaque&knowledgeBase=alpha&path=guides%2Fnote.md`);
    await viewer.locator('#okn-native-status').filter({ hasText: /ready/i }).waitFor();
    await editor.filter({ hasText: 'Native extension draft' }).waitFor();
    await viewer.locator('#okn-native-context-button').click();
    await page.locator('#context').filter({ hasText: 'guides/note.md' }).waitFor({ state: 'attached' });
    assert.equal(await viewer.locator('iframe').count(), 0);
    assert.deepEqual(errors, []);
    assert.deepEqual(appRequests, []);
    await page.goto(`http://127.0.0.1:${port}/host.html?mode=native&deepLink=${encodeURIComponent('/bases/beta?path=index.md')}`);
    await viewer.locator('.is-active-panel .note-body').filter({ hasText: 'Read-only beta document' }).waitFor();
    await viewer.locator('#okn-native-active-page').filter({ hasText: 'beta / index.md' }).waitFor();
    assert.deepEqual(errors, []);
    assert.deepEqual(appRequests, []);
    console.log('Native browser flow passed: full viewer/tree/search, images, drafts, Undo/Redo, Save, read-only bases and context under blocked frames/connections, including opaque origin.');
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
