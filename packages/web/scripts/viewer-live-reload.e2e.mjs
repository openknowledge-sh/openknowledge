import assert from "node:assert/strict";
import { execFileSync, spawn } from "node:child_process";
import { mkdtemp, mkdir, rename, rm, unlink, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { after, before, test } from "node:test";
import { chromium } from "playwright";

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..", "..");

let browser;
let viewer;
let viewerURL;
let temporary;
let wiki;
let resourcesClosed = false;

before(async () => {
  temporary = await mkdtemp(path.join(os.tmpdir(), "openknowledge-live-reload-e2e-"));
  wiki = path.join(temporary, "Wiki");
  await mkdir(path.join(wiki, "notes"), { recursive: true });
  await writeFile(path.join(wiki, "index.md"), markdown("Home", "Initial body."));
  await writeFile(path.join(wiki, "notes", "a.md"), markdown("Alpha", "Alpha body."));
  await writeFile(path.join(wiki, "notes", "b.md"), markdown("Bravo", "Bravo body."));

  const binary = path.join(temporary, "okn-live-reload-test");
  execFileSync(process.env.GO_BINARY || "go", [
    "build",
    "-o",
    binary,
    "./packages/cli/cmd/openknowledge",
  ], {
    cwd: repositoryRoot,
    env: {
      ...process.env,
      GOCACHE: process.env.GOCACHE || path.join(temporary, "go-cache"),
    },
    stdio: "pipe",
  });
  const started = startViewer(binary, wiki);
  viewer = started.child;
  viewerURL = await started.ready;
  browser = await chromium.launch();
});

after(closeResources);

async function closeResources() {
  if (resourcesClosed) return;
  resourcesClosed = true;
  await browser?.close();
  if (viewer && viewer.exitCode === null) {
    const exited = new Promise((resolve) => viewer.once("exit", resolve));
    viewer.kill("SIGTERM");
    const stopped = await Promise.race([
      exited.then(() => true),
      new Promise((resolve) => setTimeout(() => resolve(false), 5000)),
    ]);
    if (!stopped && viewer.exitCode === null) {
      viewer.kill("SIGKILL");
      await exited;
    }
  }
  if (temporary) {
    await rm(temporary, { recursive: true, force: true });
  }
}

test("local viewer reloads stable content changes and reconciles deleted panels", async () => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.addInitScript(() => {
    const key = "openknowledge.liveReloadTest.loads";
    sessionStorage.setItem(key, String(Number(sessionStorage.getItem(key) || 0) + 1));
  });
  await page.goto(viewerURL, { waitUntil: "domcontentloaded" });
  await page.getByRole("heading", { name: "Home", exact: true }).waitFor();
  await page.waitForTimeout(400);

  const initialLoads = await loadCount(page);
  await writeFile(path.join(wiki, "index.md"), markdown("Updated Home", "First update."));
  await page.getByRole("heading", { name: "Updated Home", exact: true }).waitFor();
  assert.equal(await loadCount(page), initialLoads + 1, "one stable edit must cause one page reload");

  const beforeBurst = await loadCount(page);
  await writeFile(path.join(wiki, "index.md"), markdown("Burst One", "First burst value."));
  await writeFile(path.join(wiki, "index.md"), markdown("Burst Two", "Second burst value."));
  await writeFile(path.join(wiki, "index.md"), markdown("Burst Three", "Final burst value."));
  await page.getByRole("heading", { name: "Burst Three", exact: true }).waitFor();
  await page.waitForTimeout(700);
  assert.equal(await loadCount(page), beforeBurst + 1, "one save burst must cause one page reload");

  await writeFile(path.join(wiki, "notes", "added.md"), markdown("Added", "Searchable live addition."));
  await page.locator('[data-tree-path="notes/added.md"]').waitFor({ state: "attached" });
  await page.locator("#viewer-search").fill("Searchable live addition");
  await page.locator('.search-result[href*="notes/added.md"]').waitFor();
  await page.keyboard.press("Escape");

  await page.locator("[data-sidebar-toggle]").click();
  const notesDirectory = page.locator('[data-tree-directory-path="notes"]');
  if (await notesDirectory.getAttribute("aria-expanded") !== "true") {
    await notesDirectory.click();
  }
  await page.locator('[data-tree-path="notes/a.md"]').click();
  await page.locator('[data-tree-path="notes/b.md"]').click();
  await page.locator('[data-note-path="notes/a.md"]').click();
  assert.match(page.url(), /stack=notes%2Fa\.md/);
  assert.match(page.url(), /stack=notes%2Fb\.md/);

  await unlink(path.join(wiki, "notes", "b.md"));
  await page.locator('[data-note-path="notes/b.md"]').waitFor({ state: "detached" });
  assert.equal(await page.locator('[data-note-path="notes/a.md"]').getAttribute("data-active-panel"), "true");
  assert.doesNotMatch(page.url(), /notes%2Fb\.md/);

  await rename(path.join(wiki, "notes", "a.md"), path.join(wiki, "notes", "renamed.md"));
  await page.locator('[data-tree-path="notes/renamed.md"]').waitFor({ state: "attached" });
  await page.locator('[data-note-path="notes/a.md"]').waitFor({ state: "detached" });
  assert.equal(await page.locator('[data-note-path="index.md"]').getAttribute("data-active-panel"), "true");
  assert.deepEqual(errors, []);
  await context.close();
});

test("live reload browser resources close cleanly", closeResources);

function markdown(title, body) {
  return `---\ntype: Guide\ntitle: ${title}\n---\n\n# ${title}\n\n${body}\n`;
}

async function loadCount(page) {
  return page.evaluate(() => Number(sessionStorage.getItem("openknowledge.liveReloadTest.loads") || 0));
}

function startViewer(binary, root) {
  const child = spawn(binary, ["view", "--no-browser", root], {
    cwd: repositoryRoot,
    env: { ...process.env, OPENKNOWLEDGE_TELEMETRY_SUPPRESS: "1" },
    stdio: ["ignore", "pipe", "pipe"],
  });
  let output = "";
  const ready = new Promise((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error(`viewer did not start:\n${output}`)), 15000);
    child.once("error", reject);
    child.once("exit", (code) => reject(new Error(`viewer exited before startup (${code}):\n${output}`)));
    const consume = (chunk) => {
      output += chunk.toString();
      const match = output.match(/Open Knowledge view: (https?:\/\/\S+)/);
      if (match) {
        clearTimeout(timeout);
        resolve(match[1]);
      }
    };
    child.stdout.on("data", consume);
    child.stderr.on("data", consume);
  });
  return { child, ready };
}
