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
  await writeFile(path.join(wiki, ".openknowledge.toml"), "[publish]\nenabled = true\n");
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
    "generated: { by: process:browser-e2e, at: 2026-08-03T08:00:00Z }",
    "verified: { by: human:reviewer, at: 2026-08-03T09:00:00Z }",
    "status: stable",
    "stale_after: 2027-08-03",
    "sources:",
    "  - id: rollback-policy",
    "    resource: https://example.test/rollback-policy",
    "    title: Rollback policy",
    "---",
    "",
    "# Rollback Guide",
    "",
    "Validate the deployment, capture evidence, and execute the rollback checklist.[^rollback-policy]",
    "",
    "[^rollback-policy]: Rollback policy source.",
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
  await assertSemanticPage(page, "Build a knowledge base for people and AI agents.");
  assert.equal(await page.title(), "Open Knowledge - Markdown Knowledge Bases for AI Agents");
  assert.equal(await page.locator('link[rel="canonical"]').getAttribute("href"), "https://openknowledge.sh/");
  assert.match(await page.locator('meta[name="description"]').getAttribute("content"), /AI-ready knowledge bases in Markdown/);
  await page.getByText("Ready for Codex, Claude, Cursor, or any agent.").waitFor();
  const release = page.getByRole("link", { name: /Latest Open Knowledge release v0\.8\.4/ });
  await release.waitFor();
  const setupPrompt = page.getByRole("button", { name: "Copy setup prompt" });
  await setupPrompt.click();
  await page.getByText("Paste the prompt into your agent to begin.").waitFor();
  const clipboard = await page.evaluate(() => navigator.clipboard.readText());
  assert.match(clipboard, /curl -fsSL https:\/\/openknowledge\.sh\/install \| bash/);
  assert.match(clipboard, /okn version/);
  assert.match(clipboard, /run: okn setup --prompt/);
  assert.match(clipboard, /okn validate and okn setup complete/);
  assert.match(clipboard, /purpose, audience, sources, structure, and maintenance needs/);
  assert.equal(await page.getByRole("heading", { name: "Write", exact: true }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "Verify", exact: true }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "Share", exact: true }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "One knowledge base. Three ways in." }).count(), 1);
  const core = page.getByRole("region", { name: "Core" });
  assert.match(await core.innerText(), /okn setup/);
  assert.match(await core.innerText(), /okn validate/);
  assert.doesNotMatch(await core.innerText(), /okn search/);
  const agents = page.getByRole("region", { name: "For agents" });
  assert.match(await agents.innerText(), /okn list/);
  assert.match(await agents.innerText(), /okn get/);
  assert.match(await agents.innerText(), /okn search/);
  assert.match(await agents.innerText(), /okn mcp/);
  const humans = page.getByRole("region", { name: "For humans" });
  assert.match(await humans.innerText(), /okn view/);
  const useCases = page.locator(".use-cases");
  assert.equal(await page.getByRole("heading", { name: "From Markdown to shared context." }).count(), 1);
  assert.equal(await useCases.getByRole("heading", { name: "Create a knowledge base" }).count(), 1);
  assert.equal(await useCases.getByRole("heading", { name: "Build from another source" }).count(), 1);
  assert.equal(await useCases.getByRole("heading", { name: "Connect and retrieve" }).count(), 1);
  assert.equal(await useCases.getByRole("heading", { name: "Validate before use" }).count(), 1);
  assert.equal(await useCases.getByRole("heading", { name: "Publish or integrate" }).count(), 1);
  assert.match(await useCases.innerText(), /okn setup Wiki --prompt --from https:\/\/openknowledge\.sh\/wiki\/ --depth 2/);
  assert.match(await useCases.innerText(), /okn search project "validation workflow"/);
  assert.match(await useCases.innerText(), /okn export tar --out \.\/project-wiki\.tar\.gz Wiki/);
  assert.equal(await page.getByRole("heading", { name: "Your knowledge stays yours." }).count(), 1);
  const closingGitHub = page.getByRole("link", { name: "Explore on GitHub" });
  assert.equal(await closingGitHub.getAttribute("href"), "https://github.com/openknowledge-sh/openknowledge");
  assert.equal(await closingGitHub.locator("svg").count(), 1);
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

test("exported viewer resolves OKF 0.2 source references", async () => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(new URL("guides/rollback.html", viewerURL).href, { waitUntil: "networkidle" });
  const signals = page.locator("[data-okf02-signals]");
  assert.equal(await signals.count(), 1);
  assert.match(await signals.innerText(), /Human reviewed/);
  assert.match(await signals.innerText(), /Current until 2027-08-03/);
  const ledger = signals.locator("[data-source-ledger]");
  assert.equal(await ledger.getAttribute("open"), null);
  const reference = page.getByRole("link", { name: "Source rollback-policy" });
  assert.equal(await reference.count(), 1);
  await reference.click();
  assert.equal(await ledger.getAttribute("open"), "");
  assert.equal(await ledger.locator("#ok-source-rollback-policy").count(), 1);
  assert.equal(errors.length, 0, `viewer OKF 0.2 browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("exported viewer keeps note navigation, explorer context, and settings discoverable", async () => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(viewerURL, { waitUntil: "networkidle" });
  const navigationMode = page.locator("[data-navigation-mode-toggle]");
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "beside");
  assert.equal(await navigationMode.getAttribute("aria-pressed"), "true");
  await navigationMode.click();
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "replace");
  assert.equal(await navigationMode.getAttribute("aria-pressed"), "false");
  const rollbackLink = page.getByRole("link", { name: "rollback guide" });
  await rollbackLink.click();
  await page.locator('[data-note-path="guides/rollback.md"]').waitFor({ state: "visible" });
  assert.equal(await page.locator("[data-note-path]").count(), 1, "a normal note link should replace the active panel");

  await page.goBack();
  await page.locator('[data-note-path="index.md"]').waitFor({ state: "visible" });
  await page.reload({ waitUntil: "networkidle" });
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "replace");
  await navigationMode.click();
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "beside");
  assert.equal(await navigationMode.getAttribute("aria-pressed"), "true");
  await page.reload({ waitUntil: "networkidle" });
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "beside");
  await page.getByRole("link", { name: "rollback guide" }).click();
  await page.locator('[data-note-path="guides/rollback.md"]').waitFor({ state: "visible" });
  assert.equal(await page.locator("[data-note-path]").count(), 2, "beside mode should open a normal note link beside the active panel");
  assert.equal(await page.locator("[data-note-navigator]").count(), 0, "multi-panel mode should not add a fixed bottom navigator");
  await page.locator('[data-note-path="index.md"]').click();
  assert.equal(await page.locator('[data-note-path="index.md"][data-active-panel="true"]').count(), 1);
  await page.getByRole("link", { name: "rollback guide" }).click({ modifiers: ["Shift"] });
  await page.waitForFunction(() => document.querySelectorAll("[data-note-path]").length === 1);
  assert.equal(await page.locator("[data-note-path]").count(), 1, "Shift-click should invert beside mode and replace the active panel");
  await page.goBack();
  await page.waitForFunction(() => document.querySelectorAll("[data-note-path]").length === 2);
  await page.locator('[data-note-path="index.md"]').click();

  await page.getByRole("button", { name: "Open file explorer" }).click();
  const viewport = page.viewportSize();
  await page.waitForFunction(() => {
    const header = document.querySelector("body.viewer-document > header")?.getBoundingClientRect();
    const sidebar = document.querySelector(".file-sidebar")?.getBoundingClientRect();
    const workspace = document.querySelector(".note-workspace")?.getBoundingClientRect();
    const expectedSidebarWidth = Math.max(280, Math.min(560, Math.round(window.innerWidth * 0.25)));
    return Boolean(header && sidebar && workspace
      && Math.abs(sidebar.width - expectedSidebarWidth) < 1
      && Math.abs(header.left - sidebar.right) < 1
      && Math.abs(workspace.left - sidebar.right) < 1
      && Math.abs(header.right - window.innerWidth) < 1);
  });
  const transitionLayers = await page.evaluate(async () => {
    if (typeof document.startViewTransition !== "function") return null;
    const transition = document.startViewTransition(() => {});
    await transition.ready;
    const headerGroup = getComputedStyle(document.documentElement, "::view-transition-group(viewer-header)");
    const headerNew = getComputedStyle(document.documentElement, "::view-transition-new(viewer-header)");
    const sidebarGroup = getComputedStyle(document.documentElement, "::view-transition-group(file-sidebar)");
    const sidebarNew = getComputedStyle(document.documentElement, "::view-transition-new(file-sidebar)");
    const result = {
      header: {
        zIndex: headerGroup.zIndex,
        animationName: headerNew.animationName,
        mixBlendMode: headerNew.mixBlendMode,
      },
      sidebar: {
        zIndex: sidebarGroup.zIndex,
        animationName: sidebarNew.animationName,
        mixBlendMode: sidebarNew.mixBlendMode,
      },
    };
    transition.skipTransition();
    await transition.finished;
    return result;
  });
  if (transitionLayers) {
    assert.deepEqual(transitionLayers, {
      header: {
        zIndex: "1",
        animationName: "none",
        mixBlendMode: "normal",
      },
      sidebar: {
        zIndex: "2",
        animationName: "none",
        mixBlendMode: "normal",
      },
    });
  }
  const headerBox = await page.locator("body.viewer-document > header").boundingBox();
  const sidebarBox = await page.locator(".file-sidebar").boundingBox();
  const workspaceBox = await page.locator(".note-workspace").boundingBox();
  const scrollRailBox = await page.locator(".workspace-scroll-rail").boundingBox();
  const navigationBox = await navigationMode.boundingBox();
  const settingsBox = await page.locator("[data-viewer-settings-trigger]").boundingBox();
  assert.ok(headerBox && sidebarBox && workspaceBox && scrollRailBox && navigationBox && settingsBox && viewport);
  assert.equal(Math.round(sidebarBox.width), Math.max(280, Math.min(560, Math.round(viewport.width * 0.25))), "the sidebar should default to a bounded quarter of the viewport");
  assert.equal(Math.round(headerBox.x), Math.round(sidebarBox.x + sidebarBox.width), "the header should occupy the second grid column");
  assert.equal(Math.round(workspaceBox.x), Math.round(sidebarBox.x + sidebarBox.width), "the workspace should occupy the second grid column");
  assert.equal(Math.round(scrollRailBox.x), Math.round(workspaceBox.x + 22), "the horizontal scroll rail should start inside the second grid column");
  assert.ok(scrollRailBox.x + scrollRailBox.width <= workspaceBox.x + workspaceBox.width, "the horizontal scroll rail should end inside the second grid column");
  assert.equal(Math.round(headerBox.x + headerBox.width), viewport.width, "the header should end at the viewport edge");
  assert.ok(navigationBox.x >= headerBox.x && navigationBox.x + navigationBox.width <= viewport.width, "link behavior should remain visible");
  assert.ok(settingsBox.x >= headerBox.x && settingsBox.x + settingsBox.width <= viewport.width, "viewer settings should remain visible");
  const sidebarResize = page.getByRole("separator", { name: "Resize file explorer" });
  await sidebarResize.focus();
  await sidebarResize.press("End");
  await page.waitForFunction(() => Math.abs(document.querySelector(".file-sidebar").getBoundingClientRect().width - 560) < 0.1);
  assert.equal(await sidebarResize.getAttribute("aria-valuenow"), "560");
  const resizedWorkspaceBox = await page.locator(".note-workspace").boundingBox();
  const resizedScrollRailBox = await page.locator(".workspace-scroll-rail").boundingBox();
  assert.ok(resizedWorkspaceBox && resizedScrollRailBox);
  assert.equal(Math.round(resizedScrollRailBox.x), Math.round(resizedWorkspaceBox.x + 22), "the horizontal scroll rail should follow the resized second grid column");
  assert.ok(resizedScrollRailBox.width < scrollRailBox.width, "widening the sidebar should shrink the horizontal scroll rail");
  await sidebarResize.press("Home");
  await page.waitForFunction(() => Math.abs(document.querySelector(".file-sidebar").getBoundingClientRect().width - 280) < 1);
  await sidebarResize.press("ArrowRight");
  await page.waitForFunction(() => Math.abs(document.querySelector(".file-sidebar").getBoundingClientRect().width - 304) < 1);
  assert.equal(await sidebarResize.getAttribute("aria-valuenow"), "304");
  assert.equal(await page.evaluate(() => Object.keys(localStorage).some((key) => key.startsWith("openknowledge.viewer.sidebarWidth.") && localStorage.getItem(key) === "304")), true, "the resized sidebar width should persist per knowledge graph");
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
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "beside");
  assert.equal(await navigationMode.getAttribute("aria-pressed"), "true");
  await page.getByRole("button", { name: "Viewer settings" }).click();

  await page.locator("[data-note-path]").first().locator("[data-close-panel]").click();
  await page.waitForFunction(() => document.querySelectorAll("[data-note-path]").length === 1);
  await page.locator("[data-note-path]").first().locator("[data-close-panel]").click();
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
