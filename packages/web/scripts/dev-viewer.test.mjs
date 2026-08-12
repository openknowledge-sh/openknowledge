import assert from "node:assert/strict";
import { test } from "vitest";
import { isViteViewerRequest, useViteViewerAssets } from "./dev-viewer.mjs";

test("useViteViewerAssets replaces embedded viewer assets", () => {
  const html = `
    <script src="/assets/openknowledge/viewer-theme.js"></script>
    <link rel="stylesheet" href="/assets/openknowledge/viewer.css">
    <script src="/assets/openknowledge/viewer.js"></script>
    <script src="/assets/openknowledge/viewer-live-reload.js"></script>
  `;
  const result = useViteViewerAssets(html);

  assert.match(result, /type="module" src="\/src\/viewer\/theme\.js"/);
  assert.match(result, /href="\/@fs\/.+\/viewer_theme\.css"/);
  assert.match(result, /href="\/src\/viewer\/styles\/index\.css"/);
  assert.match(result, /type="module" src="\/src\/viewer\/index\.js"/);
  assert.match(result, /type="module" src="\/src\/viewer\/live-reload-entry\.js"/);
  assert.doesNotMatch(result, /assets\/openknowledge\/viewer/);
});

test("isViteViewerRequest keeps source and runtime requests in Vite", () => {
  assert.equal(isViteViewerRequest("/@vite/client"), true);
  assert.equal(isViteViewerRequest("/src/viewer/index.js?t=1"), true);
  assert.equal(isViteViewerRequest("/node_modules/.vite/deps/mermaid.js"), true);
  assert.equal(isViteViewerRequest("/project/file/index.md"), false);
});
