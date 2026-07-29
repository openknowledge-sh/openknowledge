import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const documents = new Map([
  ["README.md", fs.readFileSync(path.join(root, "README.md"), "utf8")],
  ["packages/npm/README.md", fs.readFileSync(path.join(root, "packages", "npm", "README.md"), "utf8")],
  ["packages/npm/package.json", fs.readFileSync(path.join(root, "packages", "npm", "package.json"), "utf8")],
  [
    "packages/cli/cmd/openknowledge/command_catalog.go",
    fs.readFileSync(path.join(root, "packages", "cli", "cmd", "openknowledge", "command_catalog.go"), "utf8"),
  ],
  ["packages/web/index.html", fs.readFileSync(path.join(root, "packages", "web", "index.html"), "utf8")],
  ["Wiki/index.md", fs.readFileSync(path.join(root, "Wiki", "index.md"), "utf8")],
  [
    "Wiki/features/commands/setup.md",
    fs.readFileSync(path.join(root, "Wiki", "features", "commands", "setup.md"), "utf8"),
  ],
]);
const failures = [];
const canonicalSetup = "okn setup";
const canonicalDescription =
  "Flexible knowledge bases in Markdown that your agents can create, retrieve, validate, and publish.";
const legacyProjectSetup = /^(?:okn|openknowledge) setup Wiki --from \.$/m;
const fullCommandExample = /^[ \t]*openknowledge[ \t]+[a-z]/m;

for (const [name, content] of [
  ["README.md", documents.get("README.md")],
  ["packages/web/index.html", documents.get("packages/web/index.html")],
  ["Wiki/index.md", documents.get("Wiki/index.md")],
]) {
  if (!content.includes(canonicalSetup)) {
    failures.push(`${name} is missing the canonical setup command: ${canonicalSetup}`);
  }
  if (legacyProjectSetup.test(content)) {
    failures.push(`${name} still prescribes the legacy project setup command: okn setup Wiki --from .`);
  }
}

for (const name of [
  "README.md",
  "packages/npm/README.md",
  "packages/npm/package.json",
  "packages/cli/cmd/openknowledge/command_catalog.go",
  "packages/web/index.html",
  "Wiki/index.md",
]) {
  const normalized = documents.get(name).replace(/\s+/g, " ");
  if (!normalized.includes(canonicalDescription)) {
    failures.push(`${name} is missing the canonical product description`);
  }
}

const preferredCommandDocs = [
  ["README.md", documents.get("README.md")],
  ...collectMarkdown(path.join(root, "Wiki"))
    .filter(([name]) => name !== "Wiki/SPEC.md" && name !== "Wiki/changelog/cli.md"),
];
for (const [name, content] of preferredCommandDocs) {
  if (fullCommandExample.test(content)) {
    failures.push(`${name} uses openknowledge at the start of a command example; use okn`);
  }
}

const website = documents.get("packages/web/index.html");
if (website.includes("./project-memory")) {
  failures.push("packages/web/index.html must keep one Wiki-based onboarding path");
}
if (!website.includes("[publish] enabled = true")) {
  failures.push("packages/web/index.html must explain the explicit public-export permission");
}
if (!website.includes('data-copy-text="') || !website.includes("okn setup") || website.includes("okn setup --agent")) {
  failures.push("packages/web/index.html must copy the command that prints the primary setup prompt");
}
if (!website.includes("Get setup prompt") || website.includes("<details")) {
  failures.push("packages/web/index.html must present one setup-prompt action without an expandable prompt panel");
}
if (website.includes('<span class="tok-command">openknowledge</span>')) {
  failures.push("packages/web/index.html must use okn for shell command examples");
}

const readme = documents.get("README.md");
if (!readme.includes("[publish]\nenabled = true")) {
  failures.push("README.md must include the fail-closed publication handoff");
}
if (failures.length > 0) {
  console.error("onboarding documentation check failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exitCode = 1;
} else {
  console.log("Product entrypoints share one description and prefer printed setup with okn");
}

function collectMarkdown(directory) {
  const entries = fs.readdirSync(directory, { withFileTypes: true });
  const result = [];
  for (const entry of entries) {
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      result.push(...collectMarkdown(absolute));
      continue;
    }
    if (!entry.isFile() || path.extname(entry.name).toLowerCase() !== ".md") {
      continue;
    }
    result.push([path.relative(root, absolute).split(path.sep).join("/"), fs.readFileSync(absolute, "utf8")]);
  }
  return result;
}
