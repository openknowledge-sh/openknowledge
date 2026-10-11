import { test } from 'node:test';
import assert from 'node:assert/strict';
import { pageKey, contextForPage, parseDeepLink } from '../web/library-state.mjs';

test('identical document paths in different bases retain independent identity and context', () => {
  const a = { knowledgeBase: 'alpha', path: 'index.md', title: 'Alpha', markdown: '# Alpha', revision: 'a' };
  const b = { ...a, knowledgeBase: 'beta', markdown: '# Beta', revision: 'b' };
  assert.notEqual(pageKey(a), pageKey(b));
  assert.equal(contextForPage(b, 'Selected sentence').knowledgeBase, 'beta');
  assert.equal(contextForPage(b, 'Selected sentence').revision, 'b');
  assert.equal(contextForPage(b, 'Selected sentence').text, 'Selected sentence');
  const long = contextForPage({ ...a, markdown: 'x'.repeat(25000) });
  assert.equal(long.text.length, 24000);
  assert.equal(long.truncated, true);
});

test('deep links resolve encoded base identities and document paths without external navigation', () => {
  assert.deepEqual(parseDeepLink('/bases/design%20space?path=notes%2FA%20%26%20B.md'), { knowledgeBase: 'design space', path: 'notes/A & B.md' });
  for (const url of ['https://evil.test/bases/docs', '//evil.test/bases/docs', '/bases/%ZZ', '/bases/docs#fragment', '/other']) assert.equal(parseDeepLink(url), null);
});
