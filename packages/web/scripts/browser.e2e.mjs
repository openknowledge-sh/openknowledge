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
    "type: Index",
    "okf_bundle_title: Browser Test Handbook",
    "okf_version: \"0.2\"",
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
    "<!-- okf-annotation: agent-context -->",
    "Agent-facing maintenance context.",
    "<!-- /okf-annotation -->",
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
  await writeFile(path.join(wiki, "token-evidence.txt"), "Production tokens use the declared format.");
  await writeFile(path.join(wiki, "authentication.md"), [
    "---",
    "type: Authentication",
    "title: Authentication",
    "owner: team:identity",
    "openknowledge_claim_profile: \"1\"",
    "claim_ontology:",
    "  namespaces:",
    "    auth: https://example.test/auth/",
    "  entities:",
    "    - id: okn:service/auth",
    "      types: [okn:Service]",
    "      pref_label: Authentication service",
    "  predicates:",
    "    - id: auth:tokenFormat",
    "      object_kind: literal",
    "      datatype: xsd:string",
    "      maximum_count: 1",
    "      pref_label: token format",
    "sources:",
    "  - id: identity-openapi",
    "    resource: token-evidence.txt",
    "    observe: pinned",
    "    sha256: bb5a64e1c45b93136f128d1a3cf3d791d138709763ee26c2653ad4065f36c384",
    "    role: authoritative",
    "claims:",
    "  - id: okn:claim/token-format",
    "    slot: okn:slot/token-format",
    "    subject: okn:service/auth",
    "    predicate: auth:tokenFormat",
    "    object:",
    "      value: JWT",
    "      datatype: xsd:string",
    "    evidence:",
    "      - id: okn:evidence/token-format",
    "        source_ref: identity-openapi",
    "        stance: supports",
    "        role: primary",
    "        selector:",
    "          type: text_quote",
    "          exact: Production tokens use the declared format.",
    "    owners: [team:identity]",
    "    status: proposed",
    "    section_ref: \"#claim-token-format\"",
    "---",
    "",
    "# Authentication",
    "",
    "The production authentication service issues JSON Web Tokens.",
    "",
    '<a id="claim-token-format"></a>',
    "",
    "## Token format",
    "",
    "Production tokens use JWT.",
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
  const googleTagRequests = [];
  await page.route("**/api/telemetry", async (route) => {
    telemetry.push(JSON.parse(route.request().postData() || "{}"));
    await route.fulfill({ status: 204, body: "" });
  });
  await page.route("https://www.googletagmanager.com/gtag/js**", async (route) => {
    googleTagRequests.push(route.request().url());
    await route.fulfill({ status: 200, contentType: "text/javascript", body: "" });
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
  assert.equal(telemetry.length, 0, "first-party website telemetry must wait for consent");
  assert.equal(googleTagRequests.length, 1, "advanced consent mode must load the Google tag before consent");
  const initialGoogleCommands = await googleTagCommands(page);
  const defaultConsentIndex = initialGoogleCommands.findIndex(([command, action]) => command === "consent" && action === "default");
  const configIndex = initialGoogleCommands.findIndex(([command]) => command === "config");
  assert.ok(defaultConsentIndex >= 0 && defaultConsentIndex < configIndex, "denied consent defaults must precede Google configuration");
  assert.deepEqual(initialGoogleCommands[defaultConsentIndex][2], {
    ad_storage: "denied",
    ad_user_data: "denied",
    ad_personalization: "denied",
    analytics_storage: "denied",
  });
  assert.equal(initialGoogleCommands[configIndex][1], "G-62SWM7FC2J");
  assert.equal(await page.evaluate(() => document.cookie.split(";").some((cookie) => cookie.trim().startsWith("_ga"))), false);
  assert.equal(await page.evaluate(() => window.localStorage.getItem("openknowledge.analytics.id")), null);
  const analyticsNotice = page.getByRole("complementary", { name: "Allow analytics cookies?" });
  await analyticsNotice.waitFor({ state: "visible" });
  await analyticsNotice.getByRole("button", { name: "Allow" }).click();
  await page.waitForFunction(() => window.localStorage.getItem("openknowledge.analytics.consent") === "granted");
  await page.waitForFunction(() => window.localStorage.getItem("openknowledge.analytics.id"));
  await page.waitForFunction(() => window.dataLayer.some((entry) => entry[0] === "consent" && entry[1] === "update" && entry[2]?.analytics_storage === "granted"));
  const grantedConsent = (await googleTagCommands(page)).findLast(([command, action]) => command === "consent" && action === "update");
  assert.deepEqual(grantedConsent[2], {
    ad_storage: "denied",
    ad_user_data: "denied",
    ad_personalization: "denied",
    analytics_storage: "granted",
  });
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
  assert.ok((await googleTagCommands(page)).some(([command, eventName]) => command === "event" && eventName === "setup_prompt_copied"));
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
  assert.match(await page.locator("#closing-title").innerText(), /Your knowledge\s+stays yours/);
  const closingGitHub = page.getByRole("link", { name: "Save on GitHub" });
  assert.equal(await closingGitHub.getAttribute("href"), "https://github.com/openknowledge-sh/openknowledge");
  assert.equal(await closingGitHub.locator("svg").count(), 1);
  assert.equal(await page.locator("details").count(), 0);
  await page.getByRole("button", { name: "Analytics preferences" }).click();
  await analyticsNotice.waitFor({ state: "visible" });
  assert.equal(await page.evaluate(() => window.localStorage.getItem("openknowledge.analytics.consent")), null);
  assert.equal(await page.evaluate(() => window.localStorage.getItem("openknowledge.analytics.id")), null);
  const revokedConsent = (await googleTagCommands(page)).findLast(([command, action]) => command === "consent" && action === "update");
  assert.equal(revokedConsent[2].analytics_storage, "denied");
  assert.equal(errors.length, 0, `landing page browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("landing page keeps Google Analytics cookieless after a declined cookie choice", async () => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const telemetry = [];
  await page.route("**/api/telemetry", async (route) => {
    telemetry.push(route.request().postData());
    await route.fulfill({ status: 204, body: "" });
  });
  await page.route("https://www.googletagmanager.com/gtag/js**", (route) => route.fulfill({
    status: 200,
    contentType: "text/javascript",
    body: "",
  }));

  await page.goto(landingURL, { waitUntil: "networkidle" });
  await page.getByRole("button", { name: "No cookies" }).click();
  await page.waitForFunction(() => window.localStorage.getItem("openknowledge.analytics.consent") === "denied");
  const deniedConsent = (await googleTagCommands(page)).findLast(([command, action]) => command === "consent" && action === "update");
  assert.deepEqual(deniedConsent[2], {
    ad_storage: "denied",
    ad_user_data: "denied",
    ad_personalization: "denied",
    analytics_storage: "denied",
  });
  assert.equal(await page.evaluate(() => window.localStorage.getItem("openknowledge.analytics.id")), null);
  assert.equal(await page.evaluate(() => document.cookie.split(";").some((cookie) => cookie.trim().startsWith("_ga"))), false);
  assert.equal(telemetry.length, 0, "declining cookies must keep first-party telemetry disabled");
  await context.close();
});

test.skip("use case index links every finished guide", async () => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 960 } });
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(new URL("use-cases/", landingURL).href, { waitUntil: "networkidle" });
  await assertSemanticPage(page, "Keep the knowledge your work depends on");
  await assertASDSTE100Copy(page);
  assert.equal(await page.title(), "Open Knowledge Use Cases · Open Knowledge");
  assert.equal(
    await page.locator('link[rel="canonical"]').getAttribute("href"),
    "https://openknowledge.sh/use-cases/",
  );
  assert.equal(await page.locator('.nav-links a[href="/use-cases/"]').getAttribute("aria-current"), "page");

  const expectedGuides = [
    ["Project documentation", "/use-cases/project-documentation/"],
    ["Changelogs", "/use-cases/changelogs/"],
    ["Research notes", "/use-cases/research-notes/"],
  ];
  for (const [name, href] of expectedGuides) {
    const guide = page.locator(`.use-case-entry[href="${href}"]`);
    assert.equal(await guide.getAttribute("href"), href);
    assert.equal(await guide.getByRole("heading", { name }).count(), 1);
  }
  assert.equal(await page.getByRole("link", { name: "Set up Open Knowledge" }).getAttribute("href"), "/getting-started/");
  assert.equal(errors.length, 0, `use case index browser errors:\n${errors.join("\n")}`);

  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), true);
  await context.close();
});

test.skip("project documentation guides a durable code-and-context workflow", async () => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 960 } });
  const page = await context.newPage();
  const errors = collectPageErrors(page);
  await stubReleaseAPI(page);

  await page.goto(new URL("use-cases/project-documentation/", landingURL).href, { waitUntil: "networkidle" });
  await assertSemanticPage(page, "Help people and AI understand your project");
  await assertASDSTE100Copy(page);
  assert.equal(await page.title(), "Clear Project Documentation for People and AI · Open Knowledge");
  assert.equal(
    await page.locator('link[rel="canonical"]').getAttribute("href"),
    "https://openknowledge.sh/use-cases/project-documentation/",
  );
  assert.equal(await page.getByRole("navigation", { name: "Breadcrumb" }).count(), 1);
  assert.equal(await page.getByRole("navigation", { name: "Article contents" }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "Important project knowledge gets lost" }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "First, set up Open Knowledge" }).count(), 1);
  assert.equal(await page.getByText("mkdir example-project-docs", { exact: false }).count(), 1);
  assert.equal(await page.getByText("okn setup Wiki --interactive", { exact: false }).count(), 1);
  assert.equal(await page.getByRole("link", { name: "Read the technical setup guide" }).getAttribute("href"), "/wiki/features/commands/setup.html");
  assert.equal(await page.getByRole("heading", { name: "Start with three simple pages" }).count(), 1);
  assert.equal(await page.getByText("Architecture", { exact: true }).count(), 1);
  assert.equal(await page.getByText("Team rules", { exact: true }).count(), 1);
  assert.equal(await page.getByText("Decisions", { exact: true }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "See the same pattern in a Wikipedia example" }).count(), 1);
  assert.equal(await page.getByText("example-project-docs/", { exact: false }).count(), 1);
  const demoRepository = page.getByRole("link", { name: "Open the project documentation demo on GitHub" });
  assert.equal(
    await demoRepository.getAttribute("href"),
    "https://github.com/openknowledge-sh/example-project-docs",
  );
  assert.equal(await page.locator(".article-screenshot-placeholder").count(), 4);
  assert.equal(await page.locator(".article-screenshot-placeholder figcaption > strong", { hasText: "What to capture" }).count(), 4);
  assert.equal(await page.locator(".article-content img").count(), 0);
  assert.equal(await page.getByRole("heading", { name: "Give the AI agent context before it changes code" }).count(), 1);
  assert.equal(await page.getByText('okn search Wiki "what context affects citation validation?"', { exact: false }).count(), 1);
  assert.equal(await page.getByText("propose a Wiki update in the same change", { exact: false }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "Review the code and its explanation together" }).count(), 1);
  assert.equal(await page.getByText("okn validate Wiki", { exact: false }).count(), 1);
  assert.equal(await page.getByText("Fewer repeated explanations", { exact: true }).count(), 1);
  assert.equal(await page.getByText("Less lost context", { exact: true }).count(), 1);
  assert.equal(await page.getByText("One clear review", { exact: true }).count(), 1);
  const nextAction = page.getByRole("link", { name: "Create your first knowledge base" });
  assert.equal(await nextAction.getAttribute("href"), "/getting-started/");
  assert.deepEqual(
    await nextAction.evaluate((action) => {
      const style = getComputedStyle(action);
      return { display: style.display, alignItems: style.alignItems, justifyContent: style.justifyContent };
    }),
    { display: "flex", alignItems: "center", justifyContent: "center" },
  );
  assert.ok((await nextAction.boundingBox())?.height <= 52, "project guide CTA must stay compact");
  assert.equal(await page.locator(".site-footer").evaluate((footer) => getComputedStyle(footer).borderTopWidth), "1px");
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true);

  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true);
  assert.ok((await page.locator(".screenshot-stage").first().boundingBox())?.height < 400, "mobile screenshot placeholder must stay compact");
  assert.equal(await nextAction.isVisible(), true);
  assert.equal(errors.length, 0, `project documentation browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test.skip("changelog guide keeps user-visible changes ready for release", async () => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 960 } });
  const page = await context.newPage();
  const errors = collectPageErrors(page);
  await stubReleaseAPI(page);

  await page.goto(new URL("use-cases/changelogs/", landingURL).href, { waitUntil: "networkidle" });
  await assertSemanticPage(page, "Keep a clear record of every product change");
  await assertASDSTE100Copy(page);
  assert.equal(await page.title(), "Clear Changelogs for Every Release · Open Knowledge");
  assert.equal(
    await page.locator('link[rel="canonical"]').getAttribute("href"),
    "https://openknowledge.sh/use-cases/changelogs/",
  );
  assert.equal(await page.getByRole("navigation", { name: "Breadcrumb" }).count(), 1);
  assert.equal(await page.getByRole("navigation", { name: "Article contents" }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "Release notes should not require detective work" }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "First, set up Open Knowledge" }).count(), 1);
  assert.equal(await page.getByText("okn setup Wiki --interactive", { exact: false }).count(), 1);
  assert.equal(
    await page.getByText("okn prompt rules apply changelog --path Wiki --file AGENTS.md", { exact: false }).count(),
    1,
  );
  assert.equal(
    await page.getByRole("link", { name: "Read the technical rules guide" }).getAttribute("href"),
    "/wiki/features/commands/rules.html",
  );
  assert.equal(await page.getByRole("heading", { name: "Start with current guides and one changelog" }).count(), 1);
  assert.equal(await page.getByText("Current guides", { exact: true }).count(), 1);
  assert.equal(await page.locator(".article-foundation-list dt", { hasText: "Changelog" }).count(), 1);
  assert.equal(await page.locator(".article-foundation-list dt code", { hasText: "log.md" }).count(), 1);
  const repositoryExample = page.getByRole("link", { name: "Open the changelog demo on GitHub" });
  assert.equal(
    await repositoryExample.getAttribute("href"),
    "https://github.com/openknowledge-sh/example-changelog",
  );
  assert.equal(await page.getByRole("heading", { name: "Write for the person affected by the change" }).count(), 1);
  assert.equal(await page.getByText("Source: `packages/web/src/viewer/`", { exact: false }).count(), 1);
  assert.equal(await page.locator(".article-screenshot-placeholder").count(), 4);
  assert.equal(
    await page.locator(".article-screenshot-placeholder figcaption > strong", { hasText: "What to capture" }).count(),
    4,
  );
  assert.equal(await page.locator(".article-content img").count(), 0);
  assert.equal(await page.getByRole("heading", { name: "Ask the AI agent whether users will notice the change" }).count(), 1);
  assert.equal(await page.getByText("Decide if users will notice the change", { exact: false }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "Review the entry before you publish it" }).count(), 1);
  assert.equal(await page.getByText("okn validate Wiki", { exact: false }).count(), 1);
  assert.equal(await page.getByText("Less searching", { exact: true }).count(), 1);
  assert.equal(await page.getByText("Fewer repeated questions", { exact: true }).count(), 1);
  assert.equal(await page.getByText("One clear review", { exact: true }).count(), 1);
  const nextAction = page.getByRole("link", { name: "Create a changelog knowledge base" });
  assert.equal(await nextAction.getAttribute("href"), "/getting-started/");
  assert.deepEqual(
    await nextAction.evaluate((action) => {
      const style = getComputedStyle(action);
      return { display: style.display, alignItems: style.alignItems, justifyContent: style.justifyContent };
    }),
    { display: "flex", alignItems: "center", justifyContent: "center" },
  );
  assert.ok((await nextAction.boundingBox())?.height <= 52, "changelog guide CTA must stay compact");
  assert.equal(await page.locator(".site-footer").evaluate((footer) => getComputedStyle(footer).borderTopWidth), "1px");
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true);

  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true);
  assert.ok((await page.locator(".screenshot-stage").first().boundingBox())?.height < 400, "mobile screenshot placeholder must stay compact");
  assert.equal(await nextAction.isVisible(), true);
  assert.equal(errors.length, 0, `changelog guide browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test.skip("research notes guide keeps synthesis connected to evidence", async () => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 960 } });
  const page = await context.newPage();
  const errors = collectPageErrors(page);
  await stubReleaseAPI(page);

  await page.goto(new URL("use-cases/research-notes/", landingURL).href, { waitUntil: "networkidle" });
  await assertSemanticPage(page, "Keep every research finding connected to its sources");
  await assertASDSTE100Copy(page);
  assert.equal(await page.title(), "Simple Research Notes with Clear Sources · Open Knowledge");
  assert.equal(
    await page.locator('link[rel="canonical"]').getAttribute("href"),
    "https://openknowledge.sh/use-cases/research-notes/",
  );
  assert.equal(await page.getByRole("navigation", { name: "Breadcrumb" }).count(), 1);
  assert.equal(await page.getByRole("navigation", { name: "Article contents" }).count(), 1);
  assert.equal(
    await page.getByRole("heading", { name: "A clear answer is not enough" }).count(),
    1,
  );
  assert.equal(await page.getByRole("heading", { name: "First, set up Open Knowledge" }).count(), 1);
  assert.equal(await page.getByText("okn setup Wiki --interactive", { exact: false }).count(), 1);
  assert.equal(
    await page.getByText("okn prompt rules apply research --path Wiki --file AGENTS.md", { exact: false }).count(),
    1,
  );
  assert.equal(
    await page.getByRole("link", { name: "Read the technical rules guide" }).getAttribute("href"),
    "/wiki/features/commands/rules.html",
  );
  assert.equal(
    await page.getByRole("heading", { name: "Start with one question, source notes, and one summary" }).count(),
    1,
  );
  assert.equal(await page.getByText("Research question", { exact: true }).count(), 1);
  assert.equal(await page.getByText("Source notes", { exact: true }).count(), 1);
  assert.equal(await page.getByText("Research summary", { exact: true }).count(), 1);
  const provenanceReference = page.getByRole("link", { name: "Open the research notes demo on GitHub" });
  assert.equal(
    await provenanceReference.getAttribute("href"),
    "https://github.com/openknowledge-sh/example-research-notes",
  );
  assert.equal(await page.getByRole("heading", { name: "Show where each finding came from" }).count(), 1);
  assert.equal(await page.getByText("type: Research Note", { exact: false }).count(), 1);
  assert.equal(await page.getByText("status: draft", { exact: false }).count(), 1);
  assert.equal(await page.getByText("The current evidence does not cover mobile editing", { exact: false }).count(), 1);
  assert.equal(await page.locator(".article-screenshot-placeholder").count(), 4);
  assert.equal(
    await page.locator(".article-screenshot-placeholder figcaption > strong", { hasText: "What to capture" }).count(),
    4,
  );
  assert.equal(await page.locator(".article-content img").count(), 0);
  assert.equal(await page.getByRole("heading", { name: "Give the AI agent a simple research process" }).count(), 1);
  assert.equal(await page.getByText("Separate facts, possible conclusions, limits, and questions", { exact: false }).count(), 1);
  assert.equal(await page.getByRole("heading", { name: "Find the evidence again" }).count(), 1);
  assert.equal(
    await page.getByText('okn search Wiki "what evidence supports the citation workflow?"', { exact: false }).count(),
    1,
  );
  assert.equal(await page.getByRole("heading", { name: "Review the facts and the limits" }).count(), 1);
  assert.equal(await page.getByText("okn validate Wiki", { exact: false }).count(), 1);
  assert.equal(await page.getByText("Less repeated research", { exact: true }).count(), 1);
  assert.equal(await page.getByText("Clear limits", { exact: true }).count(), 1);
  assert.equal(await page.getByText("One clear review", { exact: true }).count(), 1);
  const nextAction = page.getByRole("link", { name: "Create a research knowledge base" });
  assert.equal(await nextAction.getAttribute("href"), "/getting-started/");
  assert.deepEqual(
    await nextAction.evaluate((action) => {
      const style = getComputedStyle(action);
      return { display: style.display, alignItems: style.alignItems, justifyContent: style.justifyContent };
    }),
    { display: "flex", alignItems: "center", justifyContent: "center" },
  );
  assert.ok((await nextAction.boundingBox())?.height <= 52, "research notes guide CTA must stay compact");
  assert.equal(await page.locator(".site-footer").evaluate((footer) => getComputedStyle(footer).borderTopWidth), "1px");
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true);

  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true);
  assert.ok((await page.locator(".screenshot-stage").first().boundingBox())?.height < 400, "mobile screenshot placeholder must stay compact");
  assert.equal(await nextAction.isVisible(), true);
  assert.equal(errors.length, 0, `research notes browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("getting started keeps completed commands visible and reusable", async () => {
  const context = await browser.newContext();
  await context.grantPermissions(["clipboard-read", "clipboard-write"], { origin: landingURL });
  const page = await context.newPage();
  const errors = collectPageErrors(page);
  await page.route("https://www.googletagmanager.com/gtag/js**", (route) => route.fulfill({
    status: 200,
    contentType: "text/javascript",
    body: "",
  }));

  await page.goto(new URL("getting-started/", landingURL).href, { waitUntil: "networkidle" });
  const gettingStartedCommands = await googleTagCommands(page);
  const gettingStartedDefaultIndex = gettingStartedCommands.findIndex(([command, action]) => command === "consent" && action === "default");
  const gettingStartedConfigIndex = gettingStartedCommands.findIndex(([command]) => command === "config");
  assert.ok(gettingStartedDefaultIndex >= 0 && gettingStartedDefaultIndex < gettingStartedConfigIndex);
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
  const quietSearchStyle = await page.locator(".header-search").evaluate((headerSearch) => {
    const input = headerSearch.querySelector(".search-input");
    const icon = headerSearch.querySelector(".search-icon");
    const shortcut = headerSearch.querySelector(".search-shortcut");
    return {
      background: getComputedStyle(input).backgroundColor,
      boxShadow: getComputedStyle(input).boxShadow,
      iconVisible: Boolean(icon && getComputedStyle(icon).display !== "none"),
      shortcutBorder: getComputedStyle(shortcut).borderTopWidth,
      shortcutBackground: getComputedStyle(shortcut).backgroundColor,
    };
  });
  assert.deepEqual(quietSearchStyle, {
    background: "rgba(0, 0, 0, 0)",
    boxShadow: "none",
    iconVisible: true,
    shortcutBorder: "0px",
    shortcutBackground: "rgba(0, 0, 0, 0)",
  });
  await search.focus();
  const resultsPanel = page.locator(".header-search .search-results");
  await resultsPanel.waitFor({ state: "visible" });
  assert.equal(await resultsPanel.getAttribute("data-mode"), "initial");
  const initialLayout = await resultsPanel.evaluate((panel) => {
    const titleRow = panel.querySelector(".search-result-title-row");
    const field = panel.closest(".header-search")?.querySelector(".search-field");
    return {
      panelWidth: panel.getBoundingClientRect().width,
      fieldWidth: field?.getBoundingClientRect().width || 0,
      titleRowDisplay: titleRow ? getComputedStyle(titleRow).display : "",
    };
  });
  assert.ok(initialLayout.panelWidth > initialLayout.fieldWidth);
  assert.equal(initialLayout.titleRowDisplay, "flex");
  await search.fill("rollback");
  const result = page.locator(".search-results a", { hasText: "Rollback Guide" }).first();
  await result.waitFor({ state: "visible" });
  assert.equal(await resultsPanel.getAttribute("data-mode"), "matches");
  const typedPanelWidth = await resultsPanel.evaluate((panel) => panel.getBoundingClientRect().width);
  assert.equal(typedPanelWidth, initialLayout.panelWidth);
  assert.ok(await result.locator(".search-result-title-row .search-result-meta").count() > 0);
  assert.ok(await result.locator("mark.search-result-highlight").count() > 0);
  assert.ok(await result.locator(".search-result-count").count() > 0);
  await search.press("ArrowDown");
  assert.equal(await page.locator('.search-results a[aria-selected="true"]').count(), 1);
  await search.press("Escape");
  assert.equal(await search.inputValue(), "");
  assert.equal(errors.length, 0, `viewer browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("exported viewer presents typed claims inline and in a responsive workspace", async () => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(new URL("authentication.html", viewerURL).href, { waitUntil: "networkidle" });
  const disclosure = page.locator("[data-claims-panel]");
  await disclosure.waitFor({ state: "visible", timeout: 5000 });
  assert.equal(await disclosure.getAttribute("open"), null);
  assert.match(await disclosure.locator(":scope > summary").innerText(), /1 statement/);
  await disclosure.locator(":scope > summary").click({ timeout: 5000 });
  await disclosure.locator(".ok-claim-subject").waitFor({ state: "visible", timeout: 5000 });
  assert.equal(await page.locator("[data-claim-section-marker]").count(), 1);

  await disclosure.locator(".ok-claim-details > summary").click({ timeout: 5000 });
  await disclosure.getByRole("button", { name: "Explore this claim" }).click({ timeout: 5000 });
  const workspace = page.locator("[data-claims-workspace]");
  await workspace.waitFor({ state: "visible", timeout: 5000 });
  assert.equal(new URL(page.url()).searchParams.get("view"), "claims");
  assert.equal(new URL(page.url()).searchParams.get("claim"), "okn:claim/token-format");
  assert.equal(await workspace.locator(".claims-results-list [role=option]").count(), 1);
  assert.equal(await workspace.getByRole("heading", { name: /Authentication service/ }).count(), 1);
  await workspace.locator('[data-claims-filter="status"]').selectOption("proposed");
  assert.equal(await workspace.locator(".claims-results-list [role=option]").count(), 1);
  await workspace.getByRole("button", { name: "Relationships", exact: true }).click({ timeout: 5000 });
  await workspace.locator(".claims-neighborhood-center").waitFor({ state: "visible", timeout: 5000 });
  await page.screenshot({ path: path.join(os.tmpdir(), "openknowledge-claims-desktop.png"), fullPage: true });

  await page.setViewportSize({ width: 390, height: 844 });
  await workspace.getByRole("button", { name: "Browse", exact: true }).click({ timeout: 5000 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true);
  assert.equal(await workspace.locator(".claims-results-list [role=option]").count(), 1);
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: path.join(os.tmpdir(), "openknowledge-claims-mobile.png"), fullPage: true });
  assert.equal(errors.length, 0, `viewer claims browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("exported viewer right-aligns search and keeps long responsive brands visible", async () => {
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
    const graphBox = await page.locator("body.viewer-document > header > .graph-view-toggle").boundingBox();
    const brandOverflow = await brand.evaluate((element) => ({
      clientWidth: element.clientWidth,
      scrollWidth: element.scrollWidth,
      textOverflow: getComputedStyle(element).textOverflow,
    }));

    assert.ok(brandBox && searchBox && graphBox, `the header controls should remain visible at ${width}px`);
    assert.equal(brandOverflow.textOverflow, "ellipsis");
    assert.ok(brandOverflow.clientWidth >= 96, `the brand should keep a useful visible width at ${width}px`);
    assert.ok(brandOverflow.clientWidth < brandOverflow.scrollWidth, `the long brand should be truncated at ${width}px`);
    assert.ok(brandBox.x + brandBox.width <= searchBox.x, `the truncated brand should not sit beneath search at ${width}px`);
    const searchGraphGap = graphBox.x - (searchBox.x + searchBox.width);
    assert.ok(searchBox.x + searchBox.width / 2 > width / 2, `search should sit in the right half at ${width}px`);
    assert.ok(searchGraphGap >= 8 && searchGraphGap <= 16, `search should align beside the graph control at ${width}px`);
  }
  await page.setViewportSize({ width: 1280, height: 844 });
  const desktopSearchBox = await page.locator(".search.header-search").boundingBox();
  assert.ok(desktopSearchBox && desktopSearchBox.width <= 380, "desktop search should keep its compact width");
  await context.close();
});

test("exported viewer resolves OKF 0.2 source references", async () => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(new URL("guides/rollback.html", viewerURL).href, { waitUntil: "networkidle" });
  const panel = page.locator('[data-note-path="guides/rollback.md"]');
  const frontmatter = page.locator("[data-frontmatter]");
  const frontmatterSummary = frontmatter.locator(":scope > summary");
  assert.equal(await frontmatter.count(), 1);
  assert.equal(await frontmatterSummary.count(), 1);
  assert.equal(await frontmatter.getAttribute("open"), null);
  assert.equal(await frontmatterSummary.isVisible(), true);
  const panelBox = await panel.boundingBox();
  const chromeBox = await panel.locator(":scope > .note-chrome").boundingBox();
  const collapsedFrontmatterBox = await frontmatter.boundingBox();
  const collapsedHeadingBox = await panel.getByRole("heading", { name: "Rollback Guide" }).boundingBox();
  assert.ok(panelBox && chromeBox && collapsedFrontmatterBox && collapsedHeadingBox);
  assert.ok(collapsedFrontmatterBox.y >= chromeBox.y + chromeBox.height - 1, "frontmatter should sit below the compact header");
  assert.ok(collapsedFrontmatterBox.width >= panelBox.width - 2, "collapsed frontmatter should use the full note width");
  assert.ok(collapsedHeadingBox.y >= collapsedFrontmatterBox.y + collapsedFrontmatterBox.height, "collapsed frontmatter should stay in normal flow before the Markdown body");
  await frontmatterSummary.focus();
  await page.keyboard.press("Enter");
  assert.equal(await frontmatter.getAttribute("open"), "");
  const expandedFrontmatterBox = await frontmatter.boundingBox();
  const expandedHeadingBox = await panel.getByRole("heading", { name: "Rollback Guide" }).boundingBox();
  assert.ok(expandedFrontmatterBox && expandedHeadingBox);
  assert.ok(expandedHeadingBox.y >= expandedFrontmatterBox.y + expandedFrontmatterBox.height, "expanded frontmatter should stay in normal flow before the Markdown body");
  const signals = frontmatter.locator("[data-okf02-signals]");
  assert.equal(await signals.count(), 1);
  assert.match(await signals.innerText(), /Human reviewed/);
  assert.match(await signals.innerText(), /Current until 2027-08-03/);
  const agentContext = panel.locator('[data-okf-annotation="agent-context"]');
  assert.equal(await agentContext.count(), 1);
  assert.equal(await agentContext.locator("details, summary").count(), 0, "agent context should not use a disclosure");
  assert.equal(await agentContext.getByText("Agent-facing maintenance context.", { exact: true }).isVisible(), true);
  const annotationColor = await agentContext.locator("p").evaluate((paragraph) => getComputedStyle(paragraph).color);
  const readerColor = await panel.locator(".note-body > p").first().evaluate((paragraph) => getComputedStyle(paragraph).color);
  assert.notEqual(annotationColor, readerColor, "agent context should use a distinct text color");
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
  assert.equal(await frontmatterSummary.isVisible(), false);
  await frontmatterVisibility.check({ force: true });
  assert.equal(await frontmatterSummary.isVisible(), true);
  assert.equal(errors.length, 0, `viewer OKF 0.2 browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("exported viewer narrates reader-facing note text with browser speech", async () => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const page = await context.newPage();
  const errors = collectPageErrors(page);
  await page.addInitScript(() => {
    class MockSpeechSynthesisUtterance {
      constructor(text) {
        this.text = text;
        this.lang = "";
        this.onend = null;
        this.onerror = null;
      }
    }
    const speechSynthesis = {
      canceled: 0,
      paused: false,
      utterances: [],
      speak(utterance) {
        this.utterances.push(utterance);
      },
      pause() {
        this.paused = true;
      },
      resume() {
        this.paused = false;
      },
      cancel() {
        this.canceled += 1;
        this.paused = false;
      },
    };
    Object.defineProperty(window, "SpeechSynthesisUtterance", { configurable: true, value: MockSpeechSynthesisUtterance });
    Object.defineProperty(window, "speechSynthesis", { configurable: true, value: speechSynthesis });
  });

  await page.goto(new URL("guides/rollback.html", viewerURL).href, { waitUntil: "networkidle" });
  const panel = page.locator('[data-note-path="guides/rollback.md"]');
  const listen = panel.locator("[data-note-narration]");
  assert.equal(await listen.isVisible(), true, "Markdown pages should expose narration when browser speech is available");
  assert.equal(await listen.getAttribute("aria-label"), "Listen to guides/rollback.md");

  await listen.click();
  assert.equal(await listen.getAttribute("aria-pressed"), "true");
  assert.equal(await listen.getAttribute("aria-label"), "Pause narration of guides/rollback.md");
  const spoken = await page.evaluate(() => window.speechSynthesis.utterances.map((utterance) => utterance.text).join(" "));
  assert.match(spoken, /Rollback Guide/);
  assert.match(spoken, /Validate the deployment/);
  assert.doesNotMatch(spoken, /Agent-facing maintenance context/);
  assert.doesNotMatch(spoken, /sequenceDiagram/);

  await listen.click();
  assert.equal(await listen.getAttribute("aria-label"), "Resume narration of guides/rollback.md");
  assert.equal(await page.evaluate(() => window.speechSynthesis.paused), true);
  await listen.click();
  assert.equal(await listen.getAttribute("aria-label"), "Pause narration of guides/rollback.md");
  assert.equal(await page.evaluate(() => window.speechSynthesis.paused), false);

  const stop = panel.getByRole("button", { name: "Stop narration of guides/rollback.md" });
  assert.equal(await stop.isVisible(), true);
  await stop.click();
  assert.equal(await listen.getAttribute("aria-pressed"), "false");
  assert.equal(await listen.getAttribute("aria-label"), "Listen to guides/rollback.md");
  assert.equal(await stop.isVisible(), false);
  assert.equal(await listen.evaluate((element) => document.activeElement === element), true, "stopping should return keyboard focus before hiding Stop");

  await listen.click();
  await page.evaluate(() => window.speechSynthesis.utterances.at(-1).onerror({ error: "interrupted" }));
  assert.equal(await listen.getAttribute("aria-pressed"), "false", "a browser speech interruption should reset narration state");
  assert.equal(await stop.isVisible(), false);

  await page.goto(viewerURL, { waitUntil: "networkidle" });
  await page.getByRole("link", { name: "rollback guide" }).click();
  const dynamicPanel = page.locator('[data-note-path="guides/rollback.md"]');
  assert.equal(await dynamicPanel.getByRole("button", { name: "Listen to guides/rollback.md" }).isVisible(), true, "dynamically opened Markdown pages should expose narration");
  assert.equal(errors.length, 0, `viewer narration browser errors:\n${errors.join("\n")}`);
  await context.close();
});

test("exported viewer keeps note navigation, explorer context, and settings discoverable", async () => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1200 } });
  const page = await context.newPage();
  const errors = collectPageErrors(page);

  await page.goto(viewerURL, { waitUntil: "networkidle" });
  const horizontalStack = page.locator("[data-horizontal-stack]");
  const graphViewControl = page.locator("header").getByRole("button", { name: "Graph view" });
  const documentsViewControl = page.getByRole("button", { name: "Documents", exact: true });
  await page.getByRole("button", { name: "Open file explorer" }).click();
  await page.locator('[data-note-path="index.md"]').evaluate((panel) => {
    panel.dataset.graphToggleTest = "preserved";
  });
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "beside");
  assert.equal(await horizontalStack.isChecked(), true);
  assert.equal(await graphViewControl.getAttribute("aria-current"), null);
  assert.equal(await graphViewControl.getAttribute("aria-pressed"), "false");
  await graphViewControl.click();
  await page.locator("[data-knowledge-graph-view] canvas").waitFor({ state: "visible" });
  assert.equal(await page.locator("html").getAttribute("data-viewer-view"), "graph");
  assert.equal(await graphViewControl.getAttribute("aria-current"), "page");
  assert.equal(await graphViewControl.getAttribute("aria-pressed"), "true");
  assert.equal(await page.locator("[data-note-path]").count(), 1, "opening graph view should preserve note panels");
  assert.equal(await page.locator("[data-note-path]").first().isVisible(), false, "graph view should temporarily hide preserved panels");
  await graphViewControl.click();
  await page.locator('[data-note-path="index.md"]').waitFor({ state: "visible" });
  assert.equal(await page.locator("html").getAttribute("data-viewer-view"), "notes");
  assert.equal(await graphViewControl.getAttribute("aria-current"), null);
  assert.equal(await graphViewControl.getAttribute("aria-pressed"), "false");
  assert.equal(await page.locator("[data-note-path]").count(), 1, "toggling graph view should restore preserved note panels");
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
  await page.getByRole("button", { name: "Viewer settings" }).click();
  await page.getByText("Horizontal stack", { exact: true }).click();
  await page.getByRole("button", { name: "Viewer settings" }).click();
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "replace");
  assert.equal(await horizontalStack.isChecked(), false);
  const rollbackLink = page.getByRole("link", { name: "rollback guide" });
  await rollbackLink.click();
  await page.locator('[data-note-path="guides/rollback.md"]').waitFor({ state: "visible" });
  assert.equal(await page.locator("[data-note-path]").count(), 1, "a normal note link should replace the active panel");

  await page.goBack();
  await page.locator('[data-note-path="index.md"]').waitFor({ state: "visible" });
  await page.reload({ waitUntil: "networkidle" });
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "replace");
  await page.getByRole("button", { name: "Open file explorer" }).click();
  await page.getByRole("button", { name: "Viewer settings" }).click();
  await page.getByText("Horizontal stack", { exact: true }).click();
  await page.getByRole("button", { name: "Viewer settings" }).click();
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "beside");
  assert.equal(await horizontalStack.isChecked(), true);
  await page.reload({ waitUntil: "networkidle" });
  assert.equal(await page.locator("html").getAttribute("data-viewer-navigation-mode"), "beside");
  await page.getByRole("link", { name: "rollback guide" }).click();
  await page.locator('[data-note-path="guides/rollback.md"]').waitFor({ state: "visible" });
  assert.equal(await page.locator("[data-note-path]").count(), 2, "beside mode should open a normal note link beside the active panel");
  const dynamicFrontmatter = page.locator('[data-note-path="guides/rollback.md"] > [data-frontmatter]');
  assert.equal(await dynamicFrontmatter.count(), 1, "dynamic note panels should place frontmatter below the header");
  assert.equal(await dynamicFrontmatter.getAttribute("open"), null, "dynamic frontmatter should start collapsed");
  assert.equal(await dynamicFrontmatter.locator(":scope > summary").isVisible(), true);
  assert.equal(await page.locator('[data-note-path="guides/rollback.md"] .note-actions > [data-frontmatter-trigger]').count(), 0, "frontmatter should not add a control to the header");
  assert.equal(await page.locator("[data-note-navigator]").count(), 0, "multi-panel mode should not add a fixed bottom navigator");
  await page.locator('[data-note-path="index.md"] .note-chrome').click();
  assert.equal(await page.locator('[data-note-path="index.md"][data-active-panel="true"]').count(), 1);
  await page.getByRole("link", { name: "rollback guide" }).click({ modifiers: ["Shift"] });
  await page.waitForFunction(() => document.querySelectorAll("[data-note-path]").length === 1);
  assert.equal(await page.locator("[data-note-path]").count(), 1, "Shift-click should invert beside mode and replace the active panel");
  await page.goBack();
  await page.waitForFunction(() => document.querySelectorAll("[data-note-path]").length === 2);
  await page.locator('[data-note-path="index.md"] .note-chrome').click();

  await page.locator("html").evaluate((html) => {
    html.dataset.viewerMotion = "full";
  });
  const workspaceScroll = page.locator(".note-workspace");
  const scrollTrack = page.locator(".workspace-scroll-track");
  const scrollThumb = page.locator(".workspace-scroll-thumb");
  const scrollTrackDragBox = await scrollTrack.boundingBox();
  const scrollThumbDragBox = await scrollThumb.boundingBox();
  assert.ok(scrollTrackDragBox && scrollThumbDragBox, "the horizontal scroll rail should be available for stacked notes");
  const dragRatio = 0.62;
  const dragStartX = scrollThumbDragBox.x + scrollThumbDragBox.width / 2;
  const dragY = scrollThumbDragBox.y + scrollThumbDragBox.height / 2;
  const dragTargetX = scrollTrackDragBox.x
    + (scrollTrackDragBox.width - scrollThumbDragBox.width) * dragRatio
    + scrollThumbDragBox.width / 2;
  await page.mouse.move(dragStartX, dragY);
  await page.mouse.down();
  assert.equal(await workspaceScroll.evaluate((element) => getComputedStyle(element).scrollBehavior), "auto", "thumb dragging should disable animated workspace scrolling");
  await page.mouse.move(dragTargetX, dragY, { steps: 4 });
  const draggedScrollRatio = await workspaceScroll.evaluate((element) => element.scrollLeft / (element.scrollWidth - element.clientWidth));
  assert.ok(Math.abs(draggedScrollRatio - dragRatio) < 0.03, "the workspace should directly track the dragged thumb position");
  await page.mouse.up();
  assert.equal(await workspaceScroll.evaluate((element) => element.classList.contains("is-rail-dragging")), false, "the workspace should leave its rail drag state after release");
  assert.equal(await workspaceScroll.evaluate((element) => getComputedStyle(element).scrollBehavior), "smooth", "non-drag workspace scrolling should preserve the full-motion preference");

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
  const documentsViewBox = await documentsViewControl.boundingBox();
  const graphViewBox = await graphViewControl.boundingBox();
  const sidebarGraphViewBox = await page.locator("[data-sidebar-graph-toggle]").boundingBox();
  const settingsBox = await page.locator("[data-viewer-settings-trigger]").boundingBox();
  assert.ok(headerBox && sidebarBox && workspaceBox && scrollRailBox && documentsViewBox && graphViewBox && sidebarGraphViewBox && settingsBox && viewport);
  assert.equal(Math.round(sidebarBox.width), Math.max(280, Math.min(560, Math.round(viewport.width * 0.25))), "the sidebar should default to a bounded quarter of the viewport");
  assert.equal(Math.round(headerBox.x), Math.round(sidebarBox.x + sidebarBox.width), "the header should occupy the second grid column");
  assert.equal(Math.round(workspaceBox.x), Math.round(sidebarBox.x + sidebarBox.width), "the workspace should occupy the second grid column");
  assert.equal(Math.round(scrollRailBox.x), Math.round(workspaceBox.x + 22), "the horizontal scroll rail should start inside the second grid column");
  assert.ok(scrollRailBox.x + scrollRailBox.width <= workspaceBox.x + workspaceBox.width, "the horizontal scroll rail should end inside the second grid column");
  assert.equal(Math.round(headerBox.x + headerBox.width), viewport.width, "the header should end at the viewport edge");
  assert.ok(documentsViewBox.x >= sidebarBox.x && documentsViewBox.x + documentsViewBox.width <= sidebarBox.x + sidebarBox.width, "Documents should stay in the sidebar");
  assert.ok(graphViewBox.x >= headerBox.x && graphViewBox.x + graphViewBox.width <= viewport.width, "Graph should remain available in the top bar");
  assert.equal(await graphViewControl.locator(".sidebar-navigation-label").isVisible(), false, "the top-bar Graph control should be icon-only");
  assert.ok(sidebarGraphViewBox.x >= sidebarBox.x && sidebarGraphViewBox.x + sidebarGraphViewBox.width <= sidebarBox.x + sidebarBox.width, "Graph should also stay in the sidebar");
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
  assert.equal(await horizontalStack.isChecked(), true);
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
  await graphCanvas.waitFor({ state: "visible" });
  const graphViewportControls = page.getByRole("toolbar", { name: "Graph viewport controls" });
  const graphSettingsToggle = graphViewportControls.getByRole("button", { name: "Graph settings" });
  assert.equal(await graphSidebar.isVisible(), false, "graph settings should be hidden by default");
  assert.equal(await graphSettingsToggle.getAttribute("aria-expanded"), "false");
  await graphSettingsToggle.click();
  await graphSidebar.waitFor({ state: "visible" });
  const graphSidebarBox = await graphSidebar.boundingBox();
  const graphCanvasBox = await graphCanvas.boundingBox();
  const graphEmptyBox = await page.locator("[data-empty-state]").boundingBox();
  assert.ok(graphSidebarBox && graphCanvasBox && graphSidebarBox.x >= graphCanvasBox.x && graphSidebarBox.y > graphCanvasBox.y, "graph settings should float over the canvas's left side");
  assert.ok(Math.abs((graphSidebarBox.y + graphSidebarBox.height / 2) - (graphCanvasBox.y + graphCanvasBox.height / 2)) < 2, "graph settings card should be vertically centered");
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
  assert.equal(await graphViewportControls.getByRole("button").count(), 4);
  assert.equal(await graphViewportControls.getByRole("button", { name: "Fit graph" }).count(), 1);
  assert.equal(await graphViewportControls.locator("button").filter({ hasText: /\S/ }).count(), 0, "graph viewport controls should be icon-only");
  assert.equal(await graphSettingsToggle.getAttribute("aria-expanded"), "true");
  await graphSettingsToggle.click();
  assert.equal(await graphSidebar.isVisible(), false, "settings control should hide the graph settings card");
  await graphSettingsToggle.click();
  assert.equal(await graphSidebar.isVisible(), true, "settings control should restore the graph settings card");
  const graphToolbarBox = await graphViewportControls.boundingBox();
  assert.ok(graphToolbarBox && graphToolbarBox.x - graphCanvasBox.x < 24 && graphCanvasBox.y + graphCanvasBox.height - graphToolbarBox.y - graphToolbarBox.height < 24, "graph controls should sit in the canvas's lower-left corner");
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
  await graphViewportControls.getByRole("button", { name: "Zoom in" }).click();
  await graphViewportControls.getByRole("button", { name: "Zoom out" }).click();
  await graphViewportControls.getByRole("button", { name: "Fit graph" }).click();
  await graphSidebar.getByRole("button", { name: "Reset graph" }).click();
  assert.equal(await graphSidebar.getByLabel("Show arrows").isChecked(), false);
  assert.equal(await graphSidebar.getByLabel("Node size").inputValue(), "100");
  assert.equal(await graphCanvas.evaluate((canvas) => canvas.width >= Math.round(canvas.getBoundingClientRect().width)), true, "graph canvas should use a responsive high-resolution backing surface");
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForFunction(() => !document.querySelector("[data-knowledge-graph-sidebar] .knowledge-graph-settings")?.open);
  assert.equal(await graphSettings.getAttribute("open"), null, "mobile graph settings should start collapsed");
  assert.equal(await graphViewportControls.getByRole("button", { name: "Fit graph" }).isVisible(), true, "viewport controls should remain available over the mobile canvas");
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
  const mobileGraphControl = page.locator("[data-file-sidebar]").getByRole("button", { name: "Graph view" });
  const mobileDocumentsControl = page.getByRole("button", { name: "Documents", exact: true });
  await page.getByRole("button", { name: "Open file explorer" }).click();
  await mobileGraphControl.click();
  const mobileGraphSidebar = page.locator("[data-knowledge-graph-sidebar]");
  const mobileViewportControls = page.getByRole("toolbar", { name: "Graph viewport controls" });
  assert.equal(await mobileGraphSidebar.isVisible(), false, "mobile graph settings should be hidden by default");
  await mobileViewportControls.getByRole("button", { name: "Graph settings" }).click();
  await mobileGraphSidebar.waitFor({ state: "visible" });
  const mobileGraphSettings = mobileGraphSidebar.locator(".knowledge-graph-settings");
  assert.equal(await mobileGraphSettings.getAttribute("open"), null, "mobile graph settings should start collapsed");
  assert.equal(await mobileGraphSidebar.getByText("Filters", { exact: true }).isVisible(), false, "collapsed mobile graph settings should hide every section");
  assert.equal(await mobileViewportControls.getByRole("button", { name: "Fit graph" }).isVisible(), true, "viewport controls should remain available over the mobile canvas");
  const collapsedSidebarBox = await mobileGraphSidebar.boundingBox();
  const mobileCanvasBox = await page.locator("[data-knowledge-graph-view] canvas").boundingBox();
  assert.ok(collapsedSidebarBox && mobileCanvasBox && mobileCanvasBox.y - collapsedSidebarBox.y - collapsedSidebarBox.height <= 24, "collapsed graph settings should not reserve empty vertical space");
  await mobileGraphSidebar.getByText("Graph settings", { exact: true }).click();
  assert.equal(await mobileGraphSettings.getAttribute("open"), "", "mobile graph settings should open as one disclosure");
  assert.equal(await mobileGraphSidebar.getByPlaceholder("Filter notes…").isVisible(), true, "open mobile graph settings should show filter controls");
  assert.equal(await mobileGraphSidebar.getByLabel("Center force").isVisible(), true, "open mobile graph settings should show force controls");
  assert.equal(await mobileGraphSidebar.getByRole("button", { name: "Reset graph" }).isVisible(), true, "open mobile graph settings should show graph actions");
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

async function assertASDSTE100Copy(page) {
  assert.equal(await page.locator("body").getAttribute("data-language-standard"), "asd-ste100");
  const violations = await page.evaluate(() => {
    const selectors = [
      ".article-deck",
      ".article-content section > p",
      ".article-content blockquote p",
      ".article-friction-list span",
      ".article-outcome-list span",
      ".article-foundation-list dd",
      ".article-repository-placeholder p",
      ".article-screenshot-placeholder figcaption p",
      ".article-setup-list > li > p",
      ".article-next > p",
    ];
    const instructionVerbs = new Set([
      "add", "apply", "ask", "create", "decide", "do", "hide", "include",
      "keep", "link", "open", "run", "save", "show", "skip", "start", "use", "write",
    ]);
    const failures = [];
    for (const element of document.querySelectorAll(selectors.join(","))) {
      const text = (element.textContent || "").replace(/\s+/g, " ").trim();
      if (text.includes(";")) failures.push(`semicolon: ${text}`);
      for (const sentence of text.split(/(?<=[.!?])\s+/)) {
        const words = sentence.match(/[A-Za-z0-9][A-Za-z0-9'’/-]*/g) || [];
        const firstWord = words[0]?.toLowerCase();
        const limit = instructionVerbs.has(firstWord) ? 20 : 25;
        if (words.length > limit) failures.push(`${words.length}/${limit} words: ${sentence}`);
      }
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

async function googleTagCommands(page) {
  return page.evaluate(() => (window.dataLayer || []).map((entry) => Array.from(entry)));
}

async function stubReleaseAPI(page) {
  await page.route("https://api.github.com/**", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({
      tag_name: "v0.8.4",
      published_at: "2026-07-28T00:00:00Z",
    }),
  }));
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
