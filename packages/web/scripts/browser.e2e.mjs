import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtemp, mkdir, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { after, before, test } from "node:test";
import { chromium } from "playwright";
import { createWebServer } from "./server.mjs";

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..", "..");
const webRoot = path.join(repositoryRoot, "packages", "web");
const webDistRoot = path.join(webRoot, "dist");

let browser;
let landingServer;
let landingURL;
let temporary;
let viewerServer;
let viewerURL;
let resourcesClosed = false;

before(async () => {
  temporary = await mkdtemp(path.join(os.tmpdir(), "openknowledge-browser-e2e-"));
  const wiki = path.join(temporary, "Wiki");
  const viewer = path.join(temporary, "viewer");
  await mkdir(path.join(wiki, "guides"), { recursive: true });
  await writeFile(path.join(wiki, "openknowledge.toml"), "[publish]\nenabled = true\n");
  await writeFile(path.join(wiki, "index.md"), [
    "---",
    "okf_bundle_title: Browser Test Handbook",
    "---",
    "",
    "# Browser Test Handbook",
    "",
    "Read the [rollback guide](guides/rollback.md).",
    "",
    "```mermaid",
    "graph LR",
    "  Handbook --> Guide",
    "```",
    "",
  ].join("\n"));
  await writeFile(path.join(wiki, "guides", "rollback.md"), [
    "---",
    "type: Guide",
    "title: Rollback Guide",
    "tags: [deployment, recovery]",
    "---",
    "",
    "# Rollback Guide",
    "",
    "Validate the deployment, capture evidence, and execute the rollback checklist.",
    "",
    "```mermaid",
    "sequenceDiagram",
    "  Operator->>System: Start rollback",
    "  System-->>Operator: Confirm recovery",
    "```",
    "",
    "```mermaid",
    "not-a-mermaid-diagram",
    "```",
    "",
  ].join("\n"));

  const go = process.env.GO_BINARY || "go";
  execFileSync(go, [
    "run",
    "./packages/cli/cmd/openknowledge",
    "export",
    "html",
    "--out",
    viewer,
    wiki,
  ], {
    cwd: repositoryRoot,
    env: {
      ...process.env,
      GOCACHE: process.env.GOCACHE || path.join(temporary, "go-cache"),
    },
    stdio: "pipe",
  });

  landingServer = createWebServer({ root: webDistRoot });
  landingURL = await listen(landingServer);
  viewerServer = createWebServer({ root: viewer });
  viewerURL = await listen(viewerServer);
  browser = await chromium.launch();
});

after(closeBrowserResources);

async function closeBrowserResources() {
  if (resourcesClosed) return;
  resourcesClosed = true;
  await browser?.close();
  await closeServer(landingServer);
  await closeServer(viewerServer);
  if (temporary) {
    await rm(temporary, { recursive: true, force: true });
  }
}

test("landing page exposes one keyboard-usable onboarding path", async () => {
  const context = await browser.newContext();
  await context.grantPermissions(["clipboard-read", "clipboard-write"], { origin: landingURL });
  const page = await context.newPage();
  const errors = collectPageErrors(page);
  await page.route("https://api.github.com/**", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({
      tag_name: "v0.8.4",
      published_at: "2026-07-28T00:00:00Z",
    }),
  }));

  await page.goto(landingURL, { waitUntil: "networkidle" });
  await assertSemanticPage(page, "Knowledge that works with your agents.");
  const setupPrompt = page.getByRole("button", { name: "Get setup prompt" });
  await setupPrompt.click();
  const clipboard = await page.evaluate(() => navigator.clipboard.readText());
  assert.match(clipboard, /okn setup(?:\n|$)/);
  assert.doesNotMatch(clipboard, /--agent/);
  assert.doesNotMatch(clipboard, /(?:okn|openknowledge) setup Wiki|--from \./);
  const githubStar = page.getByRole("link", { name: "Star us on GitHub" });
  assert.equal(await githubStar.getAttribute("href"), "https://github.com/openknowledge-sh/openknowledge");
  const compatibility = page.getByLabel("Works with Codex, Claude, and Cursor");
  assert.equal(await compatibility.count(), 1);
  assert.match(await compatibility.innerText(), /Codex/);
  assert.match(await compatibility.innerText(), /Claude/);
  assert.match(await compatibility.innerText(), /Cursor/);
  assert.equal(await page.locator("details").count(), 0);
  assert.equal(errors.length, 0, `landing page browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("exported viewer supports accessible search and keyboard navigation", async () => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(viewerURL, { waitUntil: "networkidle" });
  await assertSemanticPage(page, "Browser Test Handbook");
  const search = page.locator(".search-input");
  assert.equal(await search.count(), 1, `viewer search input missing at ${page.url()}\n${(await page.content()).slice(0, 2000)}`);
  await search.focus();
  await search.fill("rollback");
  const result = page.locator(".search-results a", { hasText: "Rollback Guide" }).first();
  await result.waitFor({ state: "visible" });
  await search.press("ArrowDown");
  assert.equal(await page.locator('.search-results a[aria-selected="true"]').count(), 1);
  await search.press("Escape");
  assert.equal(await search.inputValue(), "");
  assert.equal(errors.length, 0, `viewer browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("exported viewer keeps note navigation, explorer context, and settings discoverable", async () => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(viewerURL, { waitUntil: "networkidle" });
  const navigationMode = page.locator("[data-navigation-mode-toggle]");
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "replace");
  assert.equal(await navigationMode.getAttribute("aria-pressed"), "false");
  const rollbackLink = page.getByRole("link", { name: "rollback guide" });
  await rollbackLink.click();
  await page.locator('[data-note-path="guides/rollback.md"]').waitFor({ state: "visible" });
  assert.equal(await page.locator("[data-note-path]").count(), 1, "a normal note link should replace the active panel");

  await page.goBack();
  await page.locator('[data-note-path="index.md"]').waitFor({ state: "visible" });
  await navigationMode.click();
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "beside");
  assert.equal(await navigationMode.getAttribute("aria-pressed"), "true");
  await page.reload({ waitUntil: "networkidle" });
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "beside");
  await page.getByRole("link", { name: "rollback guide" }).click();
  await page.locator('[data-note-path="guides/rollback.md"]').waitFor({ state: "visible" });
  assert.equal(await page.locator("[data-note-path]").count(), 2, "beside mode should open a normal note link beside the active panel");
  await page.locator("[data-note-navigator]").waitFor({ state: "visible" });
  assert.equal(await page.locator(".workspace-note-tab").count(), 2);

  await page.locator(".workspace-note-tab").first().click();
  assert.equal(await page.locator('[data-note-path="index.md"][data-active-panel="true"]').count(), 1);
  await page.getByRole("link", { name: "rollback guide" }).click({ modifiers: ["Shift"] });
  await page.waitForFunction(() => document.querySelectorAll("[data-note-path]").length === 1);
  assert.equal(await page.locator("[data-note-path]").count(), 1, "Shift-click should invert beside mode and replace the active panel");
  await page.goBack();
  await page.waitForFunction(() => document.querySelectorAll("[data-note-path]").length === 2);
  await page.locator(".workspace-note-tab").first().click();

  await page.getByRole("button", { name: "Open file explorer" }).click();
  const currentFile = page.locator('.file-sidebar [data-tree-path="index.md"]');
  assert.equal(await currentFile.getAttribute("aria-current"), "page");
  await page.getByRole("button", { name: "Collapse all" }).click();
  assert.equal(await page.locator('.file-sidebar [data-tree-path="guides/rollback.md"]').isVisible(), false);
  await page.locator('.file-sidebar [data-tree-directory-path="guides"]').click();
  assert.equal(await page.locator('.file-sidebar [data-tree-path="guides/rollback.md"]').isVisible(), true);

  await page.getByRole("button", { name: "Viewer settings" }).click();
  await page.locator('[data-theme-option="default"]').click();
  await page.locator("[data-accessibility-size]").selectOption("large");
  await page.getByRole("button", { name: "Reset to defaults" }).click();
  assert.equal(await page.locator("html").getAttribute("data-viewer-theme"), "night");
  assert.equal(await page.locator("html").getAttribute("data-viewer-font-size"), "default");
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "replace");
  assert.equal(await navigationMode.getAttribute("aria-pressed"), "false");
  await page.getByRole("button", { name: "Viewer settings" }).click();

  await page.getByRole("button", { name: "Close all" }).click();
  await page.locator("[data-empty-state]").waitFor({ state: "visible" });
  assert.equal(await page.locator("[data-note-path]").count(), 0);
  assert.equal(await page.locator("[data-empty-state] .knowledge-tree").count(), 0, "the graph view should not duplicate the file explorer");
  const graphSidebar = page.locator("[data-knowledge-graph-sidebar]");
  const graphCanvas = page.locator("[data-knowledge-graph-view] canvas");
  await graphSidebar.waitFor({ state: "visible" });
  await graphCanvas.waitFor({ state: "visible" });
  const graphSidebarBox = await graphSidebar.boundingBox();
  const graphCanvasBox = await graphCanvas.boundingBox();
  assert.ok(graphSidebarBox && graphCanvasBox && graphSidebarBox.x + graphSidebarBox.width <= graphCanvasBox.x, "graph details should stay beside the canvas");
  assert.equal(errors.length, 0, `viewer navigation browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("exported viewer renders Mermaid in initial and dynamic note panels", async () => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(viewerURL, { waitUntil: "networkidle" });
  const initialDiagram = page.locator('[data-note-path="index.md"] [data-mermaid-output] svg');
  await initialDiagram.waitFor({ state: "visible" });
  assert.equal(await page.locator('[data-note-path="index.md"] [data-mermaid-source]:visible').count(), 0);

  await page.getByRole("link", { name: "rollback guide" }).click();
  const dynamicPanel = page.locator('[data-note-path="guides/rollback.md"]');
  const dynamicDiagram = dynamicPanel.locator("[data-mermaid-output] svg").first();
  await dynamicDiagram.waitFor({ state: "visible" });
  const invalidDiagram = dynamicPanel.locator('[data-mermaid-diagram][data-mermaid-state="error"]');
  await invalidDiagram.waitFor();
  assert.equal(await invalidDiagram.locator("[data-mermaid-error]:visible").count(), 1);
  assert.equal(await invalidDiagram.locator("[data-mermaid-source]:visible").count(), 1);

  await page.locator("[data-viewer-settings-trigger]").click();
  await page.locator('[data-theme-option="default"]').click();
  await page.locator('[data-note-path="guides/rollback.md"] [data-mermaid-diagram][data-mermaid-state="rendered"]').waitFor();
  assert.equal(errors.length, 0, `viewer Mermaid browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("nested exported pages work directly from file URLs", async () => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(pathToFileURL(path.join(temporary, "viewer", "guides", "rollback.html")).href, {
    waitUntil: "load",
  });
  await assertSemanticPage(page, "Rollback Guide");
  const search = page.locator(".search-input");
  await search.fill("handbook");
  await page.locator(".search-results a", { hasText: "Browser Test Handbook" }).first().waitFor({
    state: "visible",
  });
  assert.equal(errors.length, 0, `file URL viewer browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("browser resources close cleanly", closeBrowserResources);

async function assertSemanticPage(page, expectedHeading) {
  assert.equal(await page.locator("main").count(), 1, "page must expose exactly one main landmark");
  assert.equal(await page.getByRole("heading", { level: 1, name: expectedHeading }).count(), 1);
  const violations = await page.evaluate(() => {
    const failures = [];
    const ids = new Map();
    for (const element of document.querySelectorAll("[id]")) {
      const id = element.id;
      ids.set(id, (ids.get(id) || 0) + 1);
    }
    for (const [id, count] of ids) {
      if (count > 1) failures.push(`duplicate id: ${id}`);
    }
    for (const image of document.querySelectorAll("img")) {
      if (!image.hasAttribute("alt")) failures.push(`image without alt: ${image.src}`);
    }
    for (const control of document.querySelectorAll("button, input, select, textarea")) {
      const labelled = control.getAttribute("aria-label")
        || control.getAttribute("aria-labelledby")
        || control.getAttribute("title")
        || control.labels?.[0]?.textContent
        || control.textContent;
      if (!String(labelled || "").trim()) failures.push(`unlabelled control: ${control.outerHTML.slice(0, 120)}`);
    }
    return failures;
  });
  assert.deepEqual(violations, []);
}

function collectPageErrors(page) {
  const errors = [];
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(`console: ${message.text()}`);
  });
  page.on("pageerror", (error) => errors.push(`pageerror: ${error.message}`));
  return errors;
}

async function listen(server) {
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  server.unref();
  const address = server.address();
  return `http://127.0.0.1:${address.port}/`;
}

async function closeServer(server) {
  if (!server?.listening) return;
  const closed = new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  server.closeAllConnections?.();
  await closed;
}
