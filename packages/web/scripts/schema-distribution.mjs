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
  const files = await schemaFiles(cliRoot);
  if (files.length === 0) {
    throw new Error("CLI schema distribution contains no schemas");
  }
  for (const file of files) {
    const schema = JSON.parse(await readFile(path.join(cliRoot, file), "utf8"));
    const route = file.split(path.sep).join("/");
    const expected = `https://openknowledge.sh/schemas/cli/${route}`;
    if (schema.$id !== expected) {
      throw new Error(`Schema ${file} declares ${schema.$id || "no $id"}; expected ${expected}`);
    }
  }
  return files;
}

async function schemaFiles(root, relative = "") {
  const entries = await readdir(path.join(root, relative), { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const child = path.join(relative, entry.name);
    if (entry.isDirectory()) {
      files.push(...await schemaFiles(root, child));
    } else if (entry.isFile() && entry.name.endsWith(".schema.json")) {
      files.push(child);
    }
  }
  return files.sort();
}
