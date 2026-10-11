import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { Script } from 'node:vm';
import { buildApp } from '../scripts/build.mjs';

test('packaged UI is self-contained with syntactically valid JavaScript', async () => {
  await buildApp();
  const html = await readFile(new URL('../dist/app.html', import.meta.url), 'utf8');
  assert.ok(!html.includes('src="/app.js"'));
  const script = html.match(/<script>([\s\S]*?)<\/script>/)?.[1];
  assert.ok(script);
  assert.doesNotThrow(() => new Script(script));
  assert.match(html, /<style>/);
});
