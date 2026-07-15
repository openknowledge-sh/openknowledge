import { cp, mkdir, readdir, readFile, rm } from "node:fs/promises";
import path from "node:path";
import { repoRoot } from "./wiki-export.mjs";

export const schemaSource = path.join(repoRoot, "packages", "cli", "schemas");

export async function publishCLISchemas(destination) {
  const target = path.join(destination, "cli");
  await rm(target, { recursive: true, force: true });
  await mkdir(destination, { recursive: true });
  await cp(schemaSource, target, { recursive: true, errorOnExist: true });
  return validatePublishedSchemaIDs(target);
}

export async function validatePublishedSchemaIDs(cliRoot) {
  const versionRoot = path.join(cliRoot, "v1");
  const files = (await readdir(versionRoot)).filter((name) => name.endsWith(".schema.json")).sort();
  if (files.length === 0) {
    throw new Error("CLI schema distribution contains no schemas");
  }
  for (const file of files) {
    const schema = JSON.parse(await readFile(path.join(versionRoot, file), "utf8"));
    const expected = `https://openknowledge.sh/schemas/cli/v1/${file}`;
    if (schema.$id !== expected) {
      throw new Error(`Schema ${file} declares ${schema.$id || "no $id"}; expected ${expected}`);
    }
  }
  return files;
}
