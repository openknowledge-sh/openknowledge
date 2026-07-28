import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const documents = new Map([
  ["README.md", fs.readFileSync(path.join(root, "README.md"), "utf8")],
  ["packages/web/index.html", fs.readFileSync(path.join(root, "packages", "web", "index.html"), "utf8")],
  ["Wiki/index.md", fs.readFileSync(path.join(root, "Wiki", "index.md"), "utf8")],
  [
    "Wiki/features/commands/setup.md",
    fs.readFileSync(path.join(root, "Wiki", "features", "commands", "setup.md"), "utf8"),
  ],
]);
const failures = [];
const canonicalSetup = "openknowledge setup Wiki --from .";

for (const [name, content] of documents) {
  if (!content.includes(canonicalSetup)) {
    failures.push(`${name} is missing the canonical setup command: ${canonicalSetup}`);
  }
}

const website = documents.get("packages/web/index.html");
if (website.includes("./project-memory")) {
  failures.push("packages/web/index.html must keep one Wiki-based onboarding path");
}
if (!website.includes("[publish] enabled = true")) {
  failures.push("packages/web/index.html must explain the explicit public-export permission");
}

const readme = documents.get("README.md");
if (!readme.includes("[publish]\nenabled = true")) {
  failures.push("README.md must include the fail-closed publication handoff");
}
if (!readme.includes("openknowledge agent doctor --runtime <runtime>")) {
  failures.push("README.md must retain setup runtime recovery guidance");
}

if (failures.length > 0) {
  console.error("onboarding documentation check failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exitCode = 1;
} else {
  console.log("README, website, and wiki share the canonical setup and publication path");
}
