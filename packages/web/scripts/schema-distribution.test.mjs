import assert from "node:assert/strict";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { publishCLISchemas, validatePublishedSchemaIDs } from "./schema-distribution.mjs";

test("publishes every CLI schema at its declared public URL path", async () => {
  const root = await mkdtemp(path.join(os.tmpdir(), "openknowledge-schemas-"));
  try {
    const published = await publishCLISchemas(path.join(root, "schemas"));
    assert.ok(published.includes("common.schema.json"));
    assert.ok(published.includes("registry-list.schema.json"));
    assert.ok(published.includes("validation.schema.json"));

    const registry = JSON.parse(await readFile(path.join(root, "schemas", "cli", "v1", "registry-list.schema.json"), "utf8"));
    assert.equal(registry.$id, "https://openknowledge.sh/schemas/cli/v1/registry-list.schema.json");
    assert.deepEqual(await validatePublishedSchemaIDs(path.join(root, "schemas", "cli")), published);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("rejects a published schema whose id does not match its route", async () => {
  const root = await mkdtemp(path.join(os.tmpdir(), "openknowledge-schema-id-"));
  try {
    const versionRoot = path.join(root, "cli", "v1");
    await mkdir(versionRoot, { recursive: true });
    await writeFile(path.join(versionRoot, "wrong.schema.json"), JSON.stringify({ $id: "https://example.test/wrong" }));
    await assert.rejects(validatePublishedSchemaIDs(path.join(root, "cli")), /expected https:\/\/openknowledge\.sh\/schemas\/cli\/v1\/wrong\.schema\.json/);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
