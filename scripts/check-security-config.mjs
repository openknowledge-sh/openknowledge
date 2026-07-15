import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const workflow = fs.readFileSync(path.join(root, ".github", "workflows", "security.yml"), "utf8");
const dependabot = fs.readFileSync(path.join(root, ".github", "dependabot.yml"), "utf8");
const goMod = fs.readFileSync(path.join(root, "packages", "cli", "go.mod"), "utf8");
const goWork = fs.readFileSync(path.join(root, "go.work"), "utf8");
const failures = [];

const requiredWorkflowFragments = [
  "schedule:",
  "cron:",
  "workflow_dispatch:",
  "security-events: write",
  "timeout-minutes: 30",
  "language: go",
  "build-mode: manual",
  "language: javascript-typescript",
  "build-mode: none",
  "queries: security-extended",
  "go build ./...",
  "go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...",
  "https://github.com/google/osv-scanner/releases/download/v2.3.8/osv-scanner_linux_amd64",
  "OSV_SCANNER_SHA256: bc98e15319ed0d515e3f9235287ba53cdc5535d576d24fd573978ecfe9ab92dc",
  "sha256sum --check --strict",
  '"${RUNNER_TEMP}/osv-scanner" scan source --recursive --allow-no-lockfiles .',
];
for (const fragment of requiredWorkflowFragments) {
  if (!workflow.includes(fragment)) {
    failures.push(`security workflow is missing ${fragment}`);
  }
}

const init = workflow.match(/github\/codeql-action\/init@([0-9a-f]{40})/i);
const analyze = workflow.match(/github\/codeql-action\/analyze@([0-9a-f]{40})/i);
if (!init || !analyze) {
  failures.push("security workflow must pin both CodeQL init and analyze actions");
} else if (init[1] !== analyze[1]) {
  failures.push("CodeQL init and analyze actions must use the same commit");
}

for (const [name, contents] of [
  ["packages/cli/go.mod", goMod],
  ["go.work", goWork],
]) {
  if (!/^go 1\.26\.5$/m.test(contents)) {
    failures.push(`${name} must declare the reviewed Go 1.26.5 security baseline`);
  }
}

if (!/^version:\s*2\s*$/m.test(dependabot)) {
  failures.push("Dependabot configuration must use version 2");
}
const updates = dependabot.split(/\n\s+- package-ecosystem:\s*/).slice(1);
const expectedEcosystems = new Map([
  ["npm", "/"],
  ["gomod", "/packages/cli"],
  ["github-actions", "/"],
  ["docker", "/"],
]);
for (const [ecosystem, directory] of expectedEcosystems) {
  const block = updates.find((candidate) => candidate.startsWith(`"${ecosystem}"`));
  if (!block) {
    failures.push(`Dependabot is missing the ${ecosystem} ecosystem`);
    continue;
  }
  if (!block.includes(`directory: "${directory}"`)) {
    failures.push(`Dependabot ${ecosystem} directory must be ${directory}`);
  }
  if (!block.includes('interval: "weekly"')) {
    failures.push(`Dependabot ${ecosystem} schedule must be weekly`);
  }
}

if (failures.length > 0) {
  console.error("security configuration check failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exitCode = 1;
} else {
  console.log("Scheduled security scans and dependency updates are configured");
}
