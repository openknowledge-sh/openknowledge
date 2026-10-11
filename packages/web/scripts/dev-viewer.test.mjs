import assert from "node:assert/strict";
import { test } from "vitest";
import { PassThrough } from "node:stream";
import { once } from "node:events";
import { isViteViewerRequest, streamViewerResponse, useViteViewerAssets, viewerProxyHeaders } from "./dev-viewer.mjs";

for (const version of ["", "?v=0123456789abcdef"]) {
  test(`useViteViewerAssets replaces ${version ? "versioned" : "unversioned"} embedded viewer assets`, () => {
    const html = `
      <script src="/assets/openknowledge/viewer-theme.js${version}"></script>
      <link rel="stylesheet" href="/assets/openknowledge/viewer.css${version}">
      <script src="/assets/openknowledge/viewer.js${version}"></script>
      <script src="/assets/openknowledge/viewer-live-reload.js${version}"></script>
    `;
    const result = useViteViewerAssets(html);

    assert.match(result, /type="module" src="\/src\/viewer\/theme\.js"/);
    assert.match(result, /href="\/@fs\/.+\/viewer_theme\.css"/);
    assert.match(result, /href="\/src\/viewer\/styles\/index\.css"/);
    assert.match(result, /type="module" src="\/src\/viewer\/index\.js"/);
    assert.match(result, /type="module" src="\/src\/viewer\/live-reload-entry\.js"/);
    assert.doesNotMatch(result, /assets\/openknowledge\/viewer/);
  });
}

test("useViteViewerAssets leaves unrelated assets and query parameters unchanged", () => {
  const html = `
    <script src="/assets/openknowledge/viewer.js?v=invalid"></script>
    <script src="/assets/openknowledge/viewer.js?v=0123456789abcdef&amp;mode=other"></script>
    <script src="/assets/openknowledge/viewer-data.js?v=0123456789abcdef"></script>
    <script src="https://example.test/assets/openknowledge/viewer.js?v=0123456789abcdef"></script>
    <link rel="stylesheet" href="/assets/openknowledge/viewer.css?theme=night">
  `;
  assert.equal(useViteViewerAssets(html), html);
});

test("isViteViewerRequest keeps source and runtime requests in Vite", () => {
  assert.equal(isViteViewerRequest("/@vite/client"), true);
  assert.equal(isViteViewerRequest("/src/viewer/index.js?t=1"), true);
  assert.equal(isViteViewerRequest("/node_modules/.vite/deps/mermaid.js"), true);
  assert.equal(isViteViewerRequest("/project/file/index.md"), false);
});

test("viewer proxy forwards only the real development origin as the backend origin", () => {
  const target = new URL("http://127.0.0.1:12345/docs/api/document");
  assert.equal(viewerProxyHeaders({ host: "127.0.0.1:5176", origin: "http://127.0.0.1:5176" }, target).origin, target.origin);
  for (const origin of [undefined, "null", "https://evil.test", "http://127.0.0.1:5176.evil.test", "http://127.0.0.1:5176/"]) {
    assert.equal(viewerProxyHeaders({ host: "127.0.0.1:5176", origin }, target).origin, origin);
  }
});

test("viewer proxy delivers live events before the upstream stream ends", async () => {
  let controller;
  const body = new ReadableStream({ start(value) { controller = value; } });
  const response = new PassThrough();
  const firstChunk = once(response, "data");
  const copying = streamViewerResponse({ body }, response);
  controller.enqueue(new TextEncoder().encode("event: ready\ndata: {}\n\n"));
  const [chunk] = await firstChunk;
  assert.equal(chunk.toString(), "event: ready\ndata: {}\n\n");
  assert.equal(response.writableEnded, false);
  controller.close();
  await copying;
  assert.equal(response.writableEnded, true);
});
