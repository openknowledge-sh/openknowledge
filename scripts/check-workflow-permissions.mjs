import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const workflowDirectory = path.join(root, ".github", "workflows");
const expectedWriteCapabilities = new Set([
  ".github/workflows/release.yml:publish_release:contents",
  ".github/workflows/release.yml:publish_release:id-token",
  ".github/workflows/release.yml:publish_release:attestations",
  ".github/workflows/release.yml:npm:id-token",
  ".github/workflows/release.yml:runtime_images:packages",
  ".github/workflows/security.yml:codeql:security-events",
]);
const expectedPublishSteps = [
  "Checkout verified release commit",
  "Prepare release tag",
  "Run GoReleaser",
  "Attest release archives",
];
const expectedAttestationAction = "actions/attest@a1948c3f048ba23858d222213b7c278aabede763";
const expectedAttestationChecksums = "dist/checksums.txt";
const expectedVerifyPrefix = [
  "Checkout",
  "Require current default branch tip",
  "Resolve release tag",
];
const observedWriteCapabilities = new Set();
const publishSteps = [];
const verifySteps = [];
let attestationAction = "";
let attestationChecksums = "";
const failures = [];

for (const name of fs.readdirSync(workflowDirectory).sort()) {
  if (!name.endsWith(".yml") && !name.endsWith(".yaml")) {
    continue;
  }
  const relativePath = path.posix.join(".github/workflows", name);
  const lines = fs.readFileSync(path.join(workflowDirectory, name), "utf8").split(/\r?\n/);
  let inJobs = false;
  let currentJob = "";

  lines.forEach((line, index) => {
    if (/^jobs:\s*(?:#.*)?$/.test(line)) {
      inJobs = true;
      currentJob = "";
      return;
    }
    if (inJobs && /^\S/.test(line) && !/^jobs:/.test(line)) {
      inJobs = false;
      currentJob = "";
    }
    const jobMatch = inJobs ? line.match(/^  ([A-Za-z0-9_-]+):\s*(?:#.*)?$/) : null;
    if (jobMatch) {
      currentJob = jobMatch[1];
      return;
    }

    const permission = line.match(/^\s+([A-Za-z-]+):\s*write\s*(?:#.*)?$/);
    if (permission) {
      if (!currentJob) {
        failures.push(`${relativePath}:${index + 1}: write permission must be scoped to a named job`);
        return;
      }
      const capability = `${relativePath}:${currentJob}:${permission[1]}`;
      observedWriteCapabilities.add(capability);
      if (!expectedWriteCapabilities.has(capability)) {
        failures.push(`${relativePath}:${index + 1}: unexpected write capability ${permission[1]} on job ${currentJob}`);
      }
    }

    if (relativePath === ".github/workflows/release.yml" && currentJob === "publish_release") {
      const step = line.match(/^      - name:\s*(.+?)\s*$/);
      if (step) {
        publishSteps.push(step[1].replace(/^(["'])(.*)\1$/, "$2"));
      }
      const action = line.match(/^\s+uses:\s*(actions\/attest@[0-9a-f]+)\s*(?:#.*)?$/i);
      if (action) {
        attestationAction = action[1];
      }
      const checksums = line.match(/^\s+subject-checksums:\s*([^\s#]+)\s*(?:#.*)?$/);
      if (checksums) {
        attestationChecksums = checksums[1];
      }
    }
    if (relativePath === ".github/workflows/release.yml" && currentJob === "verify") {
      const step = line.match(/^      - name:\s*(.+?)\s*$/);
      if (step) {
        verifySteps.push(step[1].replace(/^(["'])(.*)\1$/, "$2"));
      }
    }
  });
}

for (const capability of expectedWriteCapabilities) {
  if (!observedWriteCapabilities.has(capability)) {
    failures.push(`missing reviewed write capability: ${capability}`);
  }
}
if (JSON.stringify(publishSteps) !== JSON.stringify(expectedPublishSteps)) {
  failures.push(`release publish job steps changed: expected ${expectedPublishSteps.join(", ")}; got ${publishSteps.join(", ")}`);
}
if (attestationAction !== expectedAttestationAction) {
  failures.push(`release attestation action changed: expected ${expectedAttestationAction}; got ${attestationAction || "none"}`);
}
if (attestationChecksums !== expectedAttestationChecksums) {
  failures.push(`release attestation checksums changed: expected ${expectedAttestationChecksums}; got ${attestationChecksums || "none"}`);
}
if (JSON.stringify(verifySteps.slice(0, expectedVerifyPrefix.length)) !== JSON.stringify(expectedVerifyPrefix)) {
  failures.push(`release verification prefix changed: expected ${expectedVerifyPrefix.join(", ")}; got ${verifySteps.slice(0, expectedVerifyPrefix.length).join(", ")}`);
}

if (failures.length > 0) {
  console.error("workflow permission check failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exitCode = 1;
} else {
  console.log("Workflow write capabilities are isolated to reviewed jobs");
}
