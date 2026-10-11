import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createResourceViewer } from '../web/resource-viewer.mjs';

test('a resource keeps its declared origin when tools come from a second MCP pool', () => {
  const resource = createResourceViewer({ viewerURL: 'http://127.0.0.1:4100/', panelURL: 'http://127.0.0.1:4101/resource-token/' });
  const page = resource.project({ knowledgeBase: 'alpha', path: 'guides/note.md', viewerURL: 'http://127.0.0.1:4200/kb/alpha/file/guides/note.md', panelURL: 'http://127.0.0.1:4201/tool-token/kb/alpha/file/guides/note.md', editorURL: 'http://127.0.0.1:4200/kb/alpha/file/guides/note.md?edit=1', revision: 'saved-revision' });
  assert.deepEqual(resource.frameDomains, ['http://127.0.0.1:4101']);
  assert.equal(page.panelURL, 'http://127.0.0.1:4101/resource-token/kb/alpha/file/guides/note.md');
  assert.equal(page.editorURL, 'http://127.0.0.1:4100/kb/alpha/file/guides/note.md?edit=1');
  assert.equal(page.panelEditorURL, 'http://127.0.0.1:4101/resource-token/kb/alpha/file/guides/note.md?edit=1');
  assert.equal(page.knowledgeBase, 'alpha');
  assert.equal(page.revision, 'saved-revision');
});

test('an explicit workspace prefix and encoded page paths survive resource projection', () => {
  const resource = createResourceViewer({ viewerURL: 'http://127.0.0.1:4100/docs/', panelURL: 'http://127.0.0.1:4101/resource-token/docs/' });
  const page = resource.project({ viewerURL: 'http://127.0.0.1:4200/docs/file/A%20%26%20B.md#section' });
  assert.equal(page.panelURL, 'http://127.0.0.1:4101/resource-token/docs/file/A%20%26%20B.md#section');
  assert.throws(() => resource.project({ viewerURL: 'http://127.0.0.1:4200/other/file/note.md' }), /outside/);
  assert.throws(() => resource.project({ viewerURL: 'https://example.com/docs/file/note.md' }), /local/);
  assert.throws(() => resource.project({ viewerURL: 'http://user:secret@127.0.0.1/docs/file/note.md' }), /local/);
});

test('resource declarations reject foreign origins and unprotected or inconsistent prefixes', () => {
  for (const bootstrap of [
    { viewerURL: 'http://127.0.0.1:4100/', panelURL: 'https://example.com/token/' },
    { viewerURL: 'http://127.0.0.1:4100/', panelURL: 'http://127.0.0.1:4101/' },
    { viewerURL: 'http://127.0.0.1:4100/docs/', panelURL: 'http://127.0.0.1:4101/token/other/' },
  ]) assert.throws(() => createResourceViewer(bootstrap));
});
