import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const semverPattern = /^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$/;

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(root, relativePath), "utf8"));
}

function fail(message) {
  console.error(`version check failed: ${message}`);
  process.exitCode = 1;
}

const rootVersion = String(readJSON("package.json").version || "").trim();
if (!semverPattern.test(rootVersion)) {
  fail(`package.json version ${JSON.stringify(rootVersion)} is not a supported release version`);
}

const requested = String(process.argv[2] || "").trim().replace(/^v/, "");
if (requested && requested !== rootVersion) {
  fail(`requested release ${requested} does not match package.json ${rootVersion}`);
}

for (const relativePath of ["packages/npm/package.json", "packages/web/package.json"]) {
  const version = String(readJSON(relativePath).version || "").trim();
  if (version !== rootVersion) {
    fail(`${relativePath} is ${version || "missing"}; expected ${rootVersion}`);
  }
}

const mainPath = "packages/cli/cmd/openknowledge/main.go";
const mainSource = fs.readFileSync(path.join(root, mainPath), "utf8");
const match = mainSource.match(/^var version = "([^"]+)"$/m);
if (!match) {
  fail(`${mainPath} does not declare the fallback CLI version`);
} else if (match[1] !== rootVersion) {
  fail(`${mainPath} is ${match[1]}; expected ${rootVersion}`);
}

if (!process.exitCode) {
  console.log(`Open Knowledge versions are aligned at ${rootVersion}`);
}
