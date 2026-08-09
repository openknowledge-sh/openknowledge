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
      OPENKNOWLEDGE_TELEMETRY_SUPPRESS: "1",
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
  const telemetry = [];
  await page.route("**/api/telemetry", async (route) => {
    telemetry.push(JSON.parse(route.request().postData() || "{}"));
    await route.fulfill({ status: 204, body: "" });
  });
  await page.route("https://api.github.com/**", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({
      tag_name: "v0.8.4",
      published_at: "2026-07-28T00:00:00Z",
    }),
  }));

  await page.goto(landingURL, { waitUntil: "networkidle" });
  assert.equal(telemetry.length, 0, "website telemetry must wait for consent");
  const analyticsNotice = page.getByRole("complementary", { name: "Anonymous website analytics" });
  await analyticsNotice.waitFor({ state: "visible" });
  await analyticsNotice.getByRole("button", { name: "Allow" }).click();
  await page.waitForFunction(() => window.localStorage.getItem("openknowledge.analytics.consent") === "granted");
  await page.waitForFunction(() => window.localStorage.getItem("openknowledge.analytics.id"));
  await page.waitForTimeout(10);
  assert.equal(telemetry.length, 1);
  assert.equal(telemetry[0].events[0].event_name, "web_page_viewed");
  await assertSemanticPage(page, "Build a knowledge base for people and AI agents.");
  assert.equal(await page.title(), "Open Knowledge - Markdown Knowledge Bases for AI Agents");
  assert.equal(await page.locator('link[rel="canonical"]').getAttribute("href"), "https://openknowledge.sh/");
  assert.match(await page.locator('meta[name="description"]').getAttribute("content"), /AI-ready knowledge bases in Markdown/);
  await page.getByText("Ready for Codex, Claude, Cursor, or any local agent.").waitFor();
  const walkthrough = page.getByRole("link", { name: "See how it works" });
  assert.equal(await walkthrough.getAttribute("href"), "/getting-started/");
  assert.equal(await walkthrough.locator('use[href="#icon-arrow-right"]').count(), 1);
  assert.deepEqual(
    await walkthrough.evaluate((link) => {
      const style = getComputedStyle(link);
      return { display: style.display, minHeight: style.minHeight, borderWidth: style.borderTopWidth };
    }),
    { display: "flex", minHeight: "50px", borderWidth: "1px" },
  );
  const release = page.getByRole("link", { name: /Latest Open Knowledge release v0\.8\.4/ });
  await release.waitFor();
  const setupPrompt = page.getByRole("button", { name: "Copy setup prompt" });
  await setupPrompt.click();
  await page.getByText("Paste the prompt into your agent to begin.").waitFor();
  await page.waitForTimeout(10);
  assert.equal(telemetry.length, 2);
  assert.equal(telemetry[1].events[0].event_name, "setup_prompt_copied");
  assert.equal(telemetry[1].events[0].interaction, "setup_prompt");
  const clipboard = await page.evaluate(() => navigator.clipboard.readText());
  assert.match(clipboard, /curl -fsSL https:\/\/openknowledge\.sh\/install\?source=homepage \| bash/);
  assert.match(clipboard, /okn version/);
  assert.match(clipboard, /run: okn setup --prompt/);
  assert.match(clipboard, /okn validate and okn setup complete/);
  assert.match(clipboard, /purpose, audience, sources, structure, and maintenance needs/);
  assert.equal(await page.getByRole("heading", { name: "Write", exact: true }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "Verify", exact: true }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "Share", exact: true }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "Use plain files" }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "Share one source" }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "Change tools without starting over" }).count(), 1);
  const guideLink = page.getByRole("link", { name: "See the 5-minute guide" });
  assert.equal(await guideLink.getAttribute("href"), "/getting-started/");
  assert.equal(await page.getByRole("heading", { name: "The idea behind Open Knowledge" }).count(), 1);
  const projectDocumentation = page.getByRole("link", { name: "project documentation" });
  assert.equal(await projectDocumentation.getAttribute("href"), "/use-cases/project-documentation/");
  assert.match(await page.locator("#closing-title").innerText(), /Your knowledge\s+stays yours/);
  const closingGitHub = page.getByRole("link", { name: "Save on GitHub" });
  assert.equal(await closingGitHub.getAttribute("href"), "https://github.com/openknowledge-sh/openknowledge");
  assert.equal(await closingGitHub.locator("svg").count(), 1);
  assert.equal(await page.locator("details").count(), 0);
  assert.equal(errors.length, 0, `landing page browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("project documentation reads as an educational Wikipedia demo", async () => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 960 } });
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(new URL("use-cases/project-documentation/", landingURL).href, { waitUntil: "networkidle" });
  await assertSemanticPage(page, "How to build project documentation that people and agents can use");
  assert.equal(await page.title(), "How to Build Useful Project Documentation · Open Knowledge");
  assert.equal(
    await page.locator('link[rel="canonical"]').getAttribute("href"),
    "https://openknowledge.sh/use-cases/project-documentation/",
  );
  assert.equal(await page.getByRole("navigation", { name: "Breadcrumb" }).count(), 1);
  assert.equal(await page.getByRole("navigation", { name: "Article contents" }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "First, set up Open Knowledge" }).count(), 1);
  assert.equal(await page.getByText("mkdir wikipedia-project-docs", { exact: false }).count(), 1);
  assert.equal(await page.getByText("okn setup Wiki --interactive", { exact: false }).count(), 1);
  assert.equal(await page.getByRole("link", { name: "Read the setup command reference" }).getAttribute("href"), "/wiki/features/commands/setup.html");
  assert.equal(await page.getByRole("heading", { name: "A concrete demo: documenting Wikipedia" }).count(), 1);
  assert.equal(await page.getByText("wikipedia-project-docs/", { exact: false }).count(), 1);
  const plannedRepository = page.locator(".article-repository-placeholder");
  assert.equal(await plannedRepository.count(), 1);
  assert.equal(await plannedRepository.locator("a").count(), 0);
  assert.match(await plannedRepository.innerText(), /planned demo repository/i);
  assert.equal(await page.locator(".article-screenshot-placeholder").count(), 4);
  assert.equal(await page.locator(".article-screenshot-placeholder figcaption > strong", { hasText: "What to capture" }).count(), 4);
  assert.equal(await page.locator(".article-content img").count(), 0);
  assert.equal(await page.getByText('okn search Wiki "how are disputed article changes discussed?"', { exact: false }).count(), 1);
  assert.equal(await page.getByText("okn validate Wiki", { exact: false }).count(), 1);
  assert.equal(await page.getByRole("link", { name: "Follow the Getting Started guide" }).getAttribute("href"), "/getting-started/");
  assert.equal(await page.locator(".site-footer").evaluate((footer) => getComputedStyle(footer).borderTopWidth), "1px");
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true);

  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true);
  assert.ok((await page.locator(".screenshot-stage").first().boundingBox())?.height < 400, "mobile screenshot placeholder must stay compact");
  assert.equal(await page.getByRole("link", { name: "Plan the knowledge architecture" }).isVisible(), true);
  assert.equal(errors.length, 0, `project documentation browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("getting started keeps completed commands visible and reusable", async () => {
  const context = await browser.newContext();
  await context.grantPermissions(["clipboard-read", "clipboard-write"], { origin: landingURL });
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(new URL("getting-started/", landingURL).href, { waitUntil: "networkidle" });
  const commands = page.locator("[data-copy-command]");
  assert.equal(await commands.count(), 5);
  assert.equal(await page.locator(".guide-command-number").count(), 0);

  const viewerFrame = page.locator(".guide-media-frame--viewer-screenshot");
  const desktopViewerBounds = await viewerFrame.boundingBox();
  assert.ok(desktopViewerBounds && Math.abs(desktopViewerBounds.width - desktopViewerBounds.height) < 1);
  assert.equal(
    await viewerFrame.locator("img").evaluate((image) => getComputedStyle(image).objectPosition),
    "36% 50%",
  );

  await page.setViewportSize({ width: 390, height: 844 });
  const mobileViewerBounds = await viewerFrame.boundingBox();
  assert.ok(mobileViewerBounds && Math.abs(mobileViewerBounds.width / mobileViewerBounds.height - 1.1) < 0.01);
  assert.equal(
    await viewerFrame.locator("img").evaluate((image) => getComputedStyle(image).objectPosition),
    "34% 50%",
  );

  const install = commands.first();
  await install.click();
  assert.equal(
    await page.evaluate(() => navigator.clipboard.readText()),
    "curl -fsSL https://openknowledge.sh/install | bash",
  );
  assert.equal(await install.getAttribute("data-state"), "copied");
  assert.equal(await install.getAttribute("data-feedback"), "copied");

  await page.waitForFunction(
    () => document.querySelector("[data-copy-command]")?.getAttribute("data-feedback") === null,
  );
  assert.equal(await install.getAttribute("data-state"), "copied");
  assert.equal(await install.getAttribute("data-feedback"), null);
  assert.equal(await install.getAttribute("aria-label"), "Copy install command again");
  await page.waitForFunction(
    () => getComputedStyle(document.querySelector(".guide-copy-icon-copy")).opacity === "1",
  );
  assert.equal(
    await install.locator(".guide-copy-icon").evaluate((icon) => getComputedStyle(icon).color),
    "rgb(7, 90, 73)",
  );
  assert.equal(errors.length, 0, `getting started browser errors:\n${errors.join("\n")}`);
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

test("exported viewer keeps long responsive brands visible before the search field", async () => {
  const context = await browser.newContext({ viewport: { width: 570, height: 844 } });
  const page = await context.newPage();

  await page.goto(viewerURL, { waitUntil: "networkidle" });
  const brand = page.locator(".header-left .brand");
  await brand.evaluate((element) => {
    element.textContent = "Open Knowledge CLI Documentation";
  });

  for (const width of [570, 690, 943]) {
    await page.setViewportSize({ width, height: 844 });
    const brandBox = await brand.boundingBox();
    const searchBox = await page.locator(".search.header-search").boundingBox();
    const brandOverflow = await brand.evaluate((element) => ({
      clientWidth: element.clientWidth,
      scrollWidth: element.scrollWidth,
      textOverflow: getComputedStyle(element).textOverflow,
    }));

    assert.ok(brandBox && searchBox, `the brand and search should remain visible at ${width}px`);
    assert.equal(brandOverflow.textOverflow, "ellipsis");
    assert.ok(brandOverflow.clientWidth >= 96, `the brand should keep a useful visible width at ${width}px`);
    assert.ok(brandOverflow.clientWidth < brandOverflow.scrollWidth, `the long brand should be truncated at ${width}px`);
    assert.ok(brandBox.x + brandBox.width <= searchBox.x, `the truncated brand should not sit beneath search at ${width}px`);
  }
  await context.close();
});

test("exported viewer resolves OKF 0.2 source references", async () => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(new URL("guides/rollback.html", viewerURL).href, { waitUntil: "networkidle" });
  const panel = page.locator('[data-note-path="guides/rollback.md"]');
  const frontmatter = page.locator("[data-frontmatter]");
  const frontmatterTrigger = panel.locator(".note-actions > [data-frontmatter-trigger]");
  assert.equal(await frontmatter.count(), 1);
  assert.equal(await frontmatterTrigger.count(), 1);
  assert.equal(await frontmatter.getAttribute("open"), null);
  assert.equal(await frontmatterTrigger.getAttribute("aria-expanded"), "false");
  assert.equal(await frontmatter.locator(":scope > summary").isHidden(), true);
  await frontmatterTrigger.focus();
  await page.keyboard.press("Enter");
  assert.equal(await frontmatter.getAttribute("open"), "");
  assert.equal(await frontmatterTrigger.getAttribute("aria-expanded"), "true");
  const panelBox = await panel.boundingBox();
  const chromeBox = await panel.locator(":scope > .note-chrome").boundingBox();
  const frontmatterBox = await frontmatter.boundingBox();
  const headingBox = await panel.getByRole("heading", { name: "Rollback Guide" }).boundingBox();
  assert.ok(panelBox && chromeBox && frontmatterBox && headingBox);
  assert.ok(frontmatterBox.y >= chromeBox.y + chromeBox.height - 1, "frontmatter should expand below the compact header");
  assert.ok(frontmatterBox.width >= panelBox.width - 2, "expanded frontmatter should use the full note width");
  assert.ok(headingBox.y >= frontmatterBox.y + frontmatterBox.height, "expanded frontmatter should stay in normal flow before the Markdown body");
  const signals = frontmatter.locator("[data-okf02-signals]");
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
  await page.getByRole("button", { name: "Open file explorer" }).click();
  await page.locator("[data-viewer-settings-trigger]").click();
  const frontmatterVisibility = page.locator("[data-frontmatter-visibility]");
  await frontmatterVisibility.uncheck({ force: true });
  assert.equal(await frontmatterTrigger.isVisible(), false);
  await frontmatterVisibility.check({ force: true });
  assert.equal(await frontmatterTrigger.isVisible(), true);
  assert.equal(errors.length, 0, `viewer OKF 0.2 browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("exported viewer keeps note navigation, explorer context, and settings discoverable", async () => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1200 } });
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(viewerURL, { waitUntil: "networkidle" });
  const navigationMode = page.locator("[data-navigation-mode-toggle]");
  const graphViewControl = page.getByRole("button", { name: "Graph view" });
  const documentsViewControl = page.getByRole("button", { name: "Documents", exact: true });
  await page.getByRole("button", { name: "Open file explorer" }).click();
  await page.locator('[data-note-path="index.md"]').evaluate((panel) => {
    panel.dataset.graphToggleTest = "preserved";
  });
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "beside");
  assert.equal(await navigationMode.getAttribute("aria-pressed"), "true");
  assert.equal(await graphViewControl.getAttribute("aria-current"), null);
  assert.equal(await graphViewControl.getAttribute("aria-pressed"), "false");
  await graphViewControl.click();
  await page.locator("[data-knowledge-graph-view] canvas").waitFor({ state: "visible" });
  assert.equal(await page.locator("html").getAttribute("data-viewer-view"), "graph");
  assert.equal(await graphViewControl.getAttribute("aria-current"), "page");
  assert.equal(await graphViewControl.getAttribute("aria-pressed"), "true");
  assert.equal(await page.locator("[data-note-path]").count(), 1, "opening graph view should preserve note panels");
  assert.equal(await page.locator("[data-note-path]").first().isVisible(), false, "graph view should temporarily hide preserved panels");
  await documentsViewControl.click();
  await page.locator('[data-note-path="index.md"]').waitFor({ state: "visible" });
  assert.equal(await page.locator("html").getAttribute("data-viewer-view"), "notes");
  assert.equal(await graphViewControl.getAttribute("aria-current"), null);
  assert.equal(await graphViewControl.getAttribute("aria-pressed"), "false");
  assert.equal(await page.locator("[data-note-path]").count(), 1, "closing graph view should restore preserved note panels");
  assert.equal(await page.locator('[data-note-path="index.md"]').getAttribute("data-graph-toggle-test"), "preserved", "closing graph view should restore the same panel element");
  await graphViewControl.click();
  await page.locator("[data-knowledge-graph-view] canvas").waitFor({ state: "visible" });
  const graphSearch = page.locator(".search-input");
  await graphSearch.fill("rollback");
  const graphSearchResult = page.locator(".search-results a", { hasText: "Rollback Guide" }).first();
  await graphSearchResult.waitFor({ state: "visible" });
  await graphSearchResult.click();
  await page.locator('[data-note-path="guides/rollback.md"]').waitFor({ state: "visible" });
  assert.equal(await page.locator("html").getAttribute("data-viewer-view"), "notes");
  assert.equal(await graphViewControl.getAttribute("aria-current"), null);
  assert.equal(await page.locator("[data-note-path]").count(), 2, "graph navigation should follow open-beside mode");
  await page.goBack();
  await page.waitForFunction(() => document.querySelectorAll("[data-note-path]").length === 1);
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
  assert.equal(await page.locator('[data-note-path="guides/rollback.md"] .note-actions > [data-frontmatter-trigger]').count(), 1, "dynamic note panels should integrate frontmatter into the header");
  assert.equal(await page.locator("[data-note-navigator]").count(), 0, "multi-panel mode should not add a fixed bottom navigator");
  await page.locator('[data-note-path="index.md"] .note-chrome').click();
  assert.equal(await page.locator('[data-note-path="index.md"][data-active-panel="true"]').count(), 1);
  await page.getByRole("link", { name: "rollback guide" }).click({ modifiers: ["Shift"] });
  await page.waitForFunction(() => document.querySelectorAll("[data-note-path]").length === 1);
  assert.equal(await page.locator("[data-note-path]").count(), 1, "Shift-click should invert beside mode and replace the active panel");
  await page.goBack();
  await page.waitForFunction(() => document.querySelectorAll("[data-note-path]").length === 2);
  await page.locator('[data-note-path="index.md"] .note-chrome').click();

  if (await page.locator("body").evaluate((body) => !body.classList.contains("is-sidebar-open"))) {
    await page.getByRole("button", { name: "Open file explorer" }).click();
  }
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
  const documentsViewBox = await documentsViewControl.boundingBox();
  const graphViewBox = await graphViewControl.boundingBox();
  const settingsBox = await page.locator("[data-viewer-settings-trigger]").boundingBox();
  assert.ok(headerBox && sidebarBox && workspaceBox && scrollRailBox && navigationBox && documentsViewBox && graphViewBox && settingsBox && viewport);
  assert.equal(Math.round(sidebarBox.width), Math.max(280, Math.min(560, Math.round(viewport.width * 0.25))), "the sidebar should default to a bounded quarter of the viewport");
  assert.equal(Math.round(headerBox.x), Math.round(sidebarBox.x + sidebarBox.width), "the header should occupy the second grid column");
  assert.equal(Math.round(workspaceBox.x), Math.round(sidebarBox.x + sidebarBox.width), "the workspace should occupy the second grid column");
  assert.equal(Math.round(scrollRailBox.x), Math.round(workspaceBox.x + 22), "the horizontal scroll rail should start inside the second grid column");
  assert.ok(scrollRailBox.x + scrollRailBox.width <= workspaceBox.x + workspaceBox.width, "the horizontal scroll rail should end inside the second grid column");
  assert.equal(Math.round(headerBox.x + headerBox.width), viewport.width, "the header should end at the viewport edge");
  assert.ok(navigationBox.x >= headerBox.x && navigationBox.x + navigationBox.width <= viewport.width, "link behavior should remain visible");
  assert.ok(documentsViewBox.x >= sidebarBox.x && documentsViewBox.x + documentsViewBox.width <= sidebarBox.x + sidebarBox.width, "Documents should stay in the sidebar");
  assert.ok(graphViewBox.x >= sidebarBox.x && graphViewBox.x + graphViewBox.width <= sidebarBox.x + sidebarBox.width, "Graph should stay in the sidebar");
  assert.ok(settingsBox.x >= sidebarBox.x && settingsBox.x + settingsBox.width <= sidebarBox.x + sidebarBox.width, "Settings should stay in the sidebar");
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
  assert.equal(await page.locator("html").getAttribute("data-viewer-theme"), "default");
  assert.equal(await page.locator("html").getAttribute("data-viewer-font-size"), "default");
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "beside");
  assert.equal(await navigationMode.getAttribute("aria-pressed"), "true");
  await page.getByRole("button", { name: "Viewer settings" }).click();

  await page.locator("[data-note-path]").first().locator("[data-close-panel]").click();
  await page.waitForFunction(() => document.querySelectorAll("[data-note-path]").length === 1);
  await page.locator("[data-note-path]").first().locator("[data-close-panel]").click();
  await page.locator("[data-empty-state]").waitFor({ state: "visible" });
  assert.equal(await page.locator("[data-note-path]").count(), 0);
  assert.equal(await graphViewControl.getAttribute("aria-pressed"), "true");
  await graphViewControl.click();
  assert.equal(await page.locator("html").getAttribute("data-viewer-view"), "graph", "graph view should remain visible when no note panels exist");
  assert.equal(await page.locator("[data-empty-state] .knowledge-tree").count(), 0, "the graph view should not duplicate the file explorer");
  const graphSidebar = page.locator("[data-knowledge-graph-sidebar]");
  const graphCanvas = page.locator("[data-knowledge-graph-view] canvas");
  await graphSidebar.waitFor({ state: "visible" });
  await graphCanvas.waitFor({ state: "visible" });
  const graphSidebarBox = await graphSidebar.boundingBox();
  const graphCanvasBox = await graphCanvas.boundingBox();
  const graphEmptyBox = await page.locator("[data-empty-state]").boundingBox();
  assert.ok(graphSidebarBox && graphCanvasBox && graphSidebarBox.x + graphSidebarBox.width <= graphCanvasBox.x, "graph details should stay beside the canvas");
  assert.ok(graphEmptyBox && Math.abs(graphCanvasBox.height - graphEmptyBox.height) < 1, "desktop graph canvas should fill the available viewer height");
  assert.equal(Math.round(graphCanvasBox.y + graphCanvasBox.height), 1200, "desktop graph canvas should reach the viewport bottom");
  assert.equal(await graphSidebar.getByRole("heading", { name: "Graph view" }).count(), 1);
  const graphSettings = graphSidebar.locator(".knowledge-graph-settings");
  assert.equal(await graphSettings.getAttribute("open"), "", "desktop graph settings should stay expanded");
  assert.equal(await graphSettings.locator(".knowledge-graph-control-section[open]").count(), 3, "desktop graph sections should keep their existing open defaults");
  assert.equal(await graphSidebar.getByPlaceholder("Filter notes…").count(), 1);
  assert.equal(await graphSidebar.getByLabel("Color nodes by folder").isChecked(), true);
  assert.equal(await graphSidebar.getByLabel("Show arrows").isChecked(), false);
  assert.equal(await graphSidebar.getByLabel("Text fade threshold").count(), 1);
  assert.equal(await graphSidebar.getByLabel("Node size").count(), 1);
  assert.equal(await graphSidebar.getByLabel("Link thickness").count(), 1);
  assert.equal(await graphSidebar.getByRole("button", { name: "Fit" }).count(), 1);
  assert.equal(await graphSidebar.getByRole("button", { name: "Reset graph" }).count(), 1);
  const animationControl = graphSidebar.locator("[data-graph-animation]");
  assert.equal(await animationControl.getAttribute("aria-pressed"), "true");
  await animationControl.click();
  assert.equal(await animationControl.textContent(), "Resume");
  assert.equal(await animationControl.getAttribute("aria-pressed"), "false");
  await graphCanvas.focus();
  const selectedGraphStatus = await graphSidebar.locator("[data-knowledge-graph-status]").textContent();
  assert.match(selectedGraphStatus, /^\d+ connections?$/, "selected graph status should show only the connection count");
  assert.doesNotMatch(selectedGraphStatus, /Enter|·/, "selected graph status should stay concise and inline");
  assert.match(await graphCanvas.getAttribute("aria-label"), /^Selected .+ with \d+ connections?\..+Enter to open\.$/, "the canvas label should retain selected-node keyboard context");
  const graphFilter = graphSidebar.getByPlaceholder("Filter notes…");
  await graphFilter.fill("rollback");
  assert.match(await graphSidebar.locator("[data-knowledge-graph-status]").textContent(), /^1 of /);
  await graphFilter.fill("");
  await graphSidebar.getByLabel("Show arrows").check();
  await graphSidebar.getByLabel("Node size").fill("125");
  assert.equal(
    await page.evaluate(() => Object.keys(localStorage).some((key) => key.startsWith("openknowledge.viewer.graphSettings.") && JSON.parse(localStorage.getItem(key)).nodeSize === 125)),
    true,
    "graph display settings should persist per knowledge graph",
  );
  await graphSidebar.getByRole("button", { name: "Zoom in" }).click();
  await graphSidebar.getByRole("button", { name: "Zoom out" }).click();
  await graphSidebar.getByRole("button", { name: "100%" }).click();
  await graphSidebar.getByRole("button", { name: "Fit" }).click();
  await graphSidebar.getByRole("button", { name: "Reset graph" }).click();
  assert.equal(await graphSidebar.getByLabel("Show arrows").isChecked(), false);
  assert.equal(await graphSidebar.getByLabel("Node size").inputValue(), "100");
  assert.equal(await graphCanvas.evaluate((canvas) => canvas.width >= Math.round(canvas.getBoundingClientRect().width)), true, "graph canvas should use a responsive high-resolution backing surface");
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForFunction(() => !document.querySelector("[data-knowledge-graph-sidebar] .knowledge-graph-settings")?.open);
  assert.equal(await graphSettings.getAttribute("open"), null, "mobile graph settings should start collapsed");
  assert.equal(await graphSidebar.getByRole("button", { name: "Fit" }).isVisible(), false, "collapsed mobile graph settings should hide all actions");
  assert.equal(Math.round((await graphCanvas.boundingBox()).height), 380, "mobile graph canvas should keep its bounded height");
  assert.equal(errors.length, 0, `viewer navigation browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("exported viewer switches sidebar notes without mobile motion", async () => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.addInitScript(() => {
    window.__openKnowledgeViewTransitionCount = 0;
    const startViewTransition = document.startViewTransition?.bind(document);
    if (!startViewTransition) return;
    document.startViewTransition = (callback) => {
      window.__openKnowledgeViewTransitionCount += 1;
      return startViewTransition(callback);
    };
  });
  await page.goto(viewerURL, { waitUntil: "networkidle" });
  const mobileGraphControl = page.getByRole("button", { name: "Graph view" });
  const mobileDocumentsControl = page.getByRole("button", { name: "Documents", exact: true });
  await page.getByRole("button", { name: "Open file explorer" }).click();
  await mobileGraphControl.click();
  const mobileGraphSidebar = page.locator("[data-knowledge-graph-sidebar]");
  await mobileGraphSidebar.waitFor({ state: "visible" });
  const mobileGraphSettings = mobileGraphSidebar.locator(".knowledge-graph-settings");
  assert.equal(await mobileGraphSettings.getAttribute("open"), null, "mobile graph settings should start collapsed");
  assert.equal(await mobileGraphSidebar.getByText("Filters", { exact: true }).isVisible(), false, "collapsed mobile graph settings should hide every section");
  assert.equal(await mobileGraphSidebar.getByRole("button", { name: "Fit" }).isVisible(), false, "collapsed mobile graph settings should hide every action");
  const collapsedSidebarBox = await mobileGraphSidebar.boundingBox();
  const mobileCanvasBox = await page.locator("[data-knowledge-graph-view] canvas").boundingBox();
  assert.ok(collapsedSidebarBox && mobileCanvasBox && mobileCanvasBox.y - collapsedSidebarBox.y - collapsedSidebarBox.height <= 24, "collapsed graph settings should not reserve empty vertical space");
  await mobileGraphSidebar.getByText("Graph settings", { exact: true }).click();
  assert.equal(await mobileGraphSettings.getAttribute("open"), "", "mobile graph settings should open as one disclosure");
  assert.equal(await mobileGraphSidebar.getByPlaceholder("Filter notes…").isVisible(), true, "open mobile graph settings should show filter controls");
  assert.equal(await mobileGraphSidebar.getByLabel("Center force").isVisible(), true, "open mobile graph settings should show force controls");
  assert.equal(await mobileGraphSidebar.getByRole("button", { name: "Fit" }).isVisible(), true, "open mobile graph settings should show every action");
  await page.getByRole("button", { name: "Open file explorer" }).click();
  await mobileDocumentsControl.click();
  await page.locator('[data-note-path="index.md"]').waitFor({ state: "visible" });
  await page.getByRole("button", { name: "Open file explorer" }).click();
  assert.equal(await page.evaluate(() => getComputedStyle(document.body).transitionDuration), "0s");

  await page.locator('.file-sidebar [data-tree-directory-path="guides"]').click();
  await page.locator('.file-sidebar [data-tree-path="guides/rollback.md"]').click();
  const panel = page.locator('[data-note-path="guides/rollback.md"]');
  await panel.waitFor({ state: "visible" });
  assert.equal(await page.locator("[data-file-sidebar]").getAttribute("aria-hidden"), "true");
  assert.equal(await panel.evaluate((element) => element.classList.contains("is-entering")), false);
  assert.equal(await page.evaluate(() => window.__openKnowledgeViewTransitionCount), 0);
  assert.equal(errors.length, 0, `viewer mobile navigation browser errors:\n${errors.join("\n")}`);
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

  await page.getByRole("button", { name: "Open file explorer" }).click();
  await page.locator("[data-viewer-settings-trigger]").click();
  await page.locator('[data-theme-option="default"]').click();
  await page.locator('[data-note-path="guides/rollback.md"] [data-mermaid-diagram][data-mermaid-state="rendered"]').waitFor();
  assert.equal(errors.length, 0, `viewer Mermaid browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("exported Mermaid diagrams open in a zoomable viewport", async () => {
  const context = await browser.newContext({ viewport: { width: 1280, height: 720 } });
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(viewerURL, { waitUntil: "networkidle" });
  const trigger = page.getByRole("button", { name: "Mermaid diagram 1 in Browser Test Handbook" });
  await trigger.click();

  const dialog = page.getByRole("dialog", { name: "Mermaid diagram 1 in Browser Test Handbook" });
  await dialog.waitFor({ state: "visible" });
  const dialogBox = await dialog.boundingBox();
  assert.deepEqual(dialogBox && { width: dialogBox.width, height: dialogBox.height }, { width: 1280, height: 720 });

  const status = page.getByRole("status", { name: "Diagram zoom" });
  const zoomIn = page.getByRole("button", { name: "Zoom in" });
  const zoomOut = page.getByRole("button", { name: "Zoom out" });
  const actual = page.getByRole("button", { name: "Show diagram at 100%" });
  const fit = page.getByRole("button", { name: "Fit diagram to viewport" });
  const canvas = page.getByLabel("Diagram canvas. Drag to pan.");
  const stage = page.locator(".ok-mermaid-viewport-stage");

  const fittedZoom = zoomPercent(await status.innerText());
  await zoomIn.click();
  assert.ok(zoomPercent(await status.innerText()) > fittedZoom);
  await zoomOut.click();
  await actual.click();
  assert.equal(await status.innerText(), "100%");
  await fit.click();
  assert.equal(zoomPercent(await status.innerText()), fittedZoom);

  await canvas.hover();
  await page.mouse.wheel(0, -100);
  assert.ok(zoomPercent(await status.innerText()) > fittedZoom);

  const canvasBox = await canvas.boundingBox();
  assert.ok(canvasBox);
  const beforeDrag = await stage.getAttribute("style");
  await page.mouse.move(canvasBox.x + canvasBox.width / 2, canvasBox.y + canvasBox.height / 2);
  await page.mouse.down();
  await page.mouse.move(canvasBox.x + canvasBox.width / 2 + 90, canvasBox.y + canvasBox.height / 2 + 50);
  await page.mouse.up();
  const afterDrag = await stage.getAttribute("style");
  assert.notEqual(afterDrag, beforeDrag);
  await canvas.press("ArrowRight");
  assert.notEqual(await stage.getAttribute("style"), afterDrag);

  await canvas.press("Escape");
  await dialog.waitFor({ state: "hidden" });
  assert.equal(await trigger.evaluate((element) => document.activeElement === element), true);
  const restoredDiagram = page.locator('[data-note-path="index.md"] [data-mermaid-output] svg');
  await restoredDiagram.waitFor({ state: "visible" });
  assert.equal(await restoredDiagram.count(), 1);

  await trigger.press("Enter");
  await dialog.waitFor({ state: "visible" });
  await page.getByRole("button", { name: "Close diagram viewer" }).click();
  await dialog.waitFor({ state: "hidden" });
  assert.equal(await trigger.evaluate((element) => document.activeElement === element), true);
  await restoredDiagram.waitFor({ state: "visible" });
  await page.keyboard.press("Space");
  await dialog.waitFor({ state: "visible" });
  await page.keyboard.press("Escape");

  assert.equal(errors.length, 0, `viewer Mermaid viewport browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("exported Mermaid viewport fits mobile screens", async () => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(viewerURL, { waitUntil: "networkidle" });
  await page.getByRole("button", { name: "Mermaid diagram 1 in Browser Test Handbook" }).click();
  const dialog = page.getByRole("dialog", { name: "Mermaid diagram 1 in Browser Test Handbook" });
  const toolbar = page.getByRole("toolbar", { name: "Diagram zoom controls" });
  const dialogBox = await dialog.boundingBox();
  const toolbarBox = await toolbar.boundingBox();
  assert.deepEqual(dialogBox && { width: dialogBox.width, height: dialogBox.height }, { width: 390, height: 844 });
  assert.ok(toolbarBox && toolbarBox.x >= 0 && toolbarBox.x + toolbarBox.width <= 390);
  for (const name of ["Zoom out", "Zoom in", "Show diagram at 100%", "Fit diagram to viewport", "Close diagram viewer"]) {
    assert.equal(await page.getByRole("button", { name }).isVisible(), true);
  }
  assert.equal(errors.length, 0, `mobile Mermaid viewport browser errors:\n${errors.join("\n")}`);
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

function zoomPercent(value) {
  return Number.parseInt(value, 10);
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
