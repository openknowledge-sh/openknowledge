import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const semverPattern = /^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$/;
const version = String(process.argv[2] || "").trim().replace(/^v/, "");

if (!semverPattern.test(version)) {
  console.error(`version update failed: ${JSON.stringify(version)} is not a supported release version`);
  process.exit(1);
}

const updates = [];
for (const relativePath of ["package.json", "packages/npm/package.json", "packages/web/package.json"]) {
  const absolutePath = path.join(root, relativePath);
  const manifest = JSON.parse(fs.readFileSync(absolutePath, "utf8"));
  manifest.version = version;
  updates.push([absolutePath, `${JSON.stringify(manifest, null, 2)}\n`]);
}

const mainPath = path.join(root, "packages/cli/cmd/openknowledge/main.go");
const mainSource = fs.readFileSync(mainPath, "utf8");
if (!/^var version = "[^"]+"$/m.test(mainSource)) {
  console.error("version update failed: packages/cli/cmd/openknowledge/main.go does not declare the fallback CLI version");
  process.exit(1);
}
updates.push([mainPath, mainSource.replace(/^var version = "[^"]+"$/m, `var version = "${version}"`)]);

for (const [absolutePath, source] of updates) {
  fs.writeFileSync(absolutePath, source);
}

console.log(`Updated Open Knowledge versions to ${version}`);
