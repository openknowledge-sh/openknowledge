import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const script = new URL("./extract-release-notes.mjs", import.meta.url);

test("extracts one dated release section", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "openknowledge-release-notes-"));
  const changelog = path.join(root, "cli.md");
  const output = path.join(root, "notes.md");
  fs.writeFileSync(changelog, [
    "## Unreleased",
    "",
    "No changes yet.",
    "",
    "## v0.13.0 — 2026-08-31",
    "",
    "Release message.",
    "",
    "### Viewer",
    "",
    "- A viewer change.",
    "",
    "## v0.12.0 — 2026-08-11",
    "",
    "Older notes.",
    "",
  ].join("\n"));

  execFileSync(process.execPath, [script.pathname, "v0.13.0", changelog, output]);
  assert.equal(fs.readFileSync(output, "utf8"), "Release message.\n\n### Viewer\n\n- A viewer change.\n");
});

test("rejects a version without dated notes", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "openknowledge-release-notes-"));
  const changelog = path.join(root, "cli.md");
  fs.writeFileSync(changelog, "## Unreleased\n\nPending.\n");

  const result = spawnSync(process.execPath, [script.pathname, "0.13.0", changelog], { encoding: "utf8" });
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /does not contain a dated v0\.13\.0 section/);
});
