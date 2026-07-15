import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const workflowDirectory = path.join(root, ".github", "workflows");
const commitPattern = /^[0-9a-f]{40}$/i;
const dockerDigestPattern = /^docker:\/\/.+@sha256:[0-9a-f]{64}$/i;
const failures = [];

for (const name of fs.readdirSync(workflowDirectory).sort()) {
  if (!name.endsWith(".yml") && !name.endsWith(".yaml")) {
    continue;
  }
  const relativePath = path.posix.join(".github/workflows", name);
  const lines = fs.readFileSync(path.join(workflowDirectory, name), "utf8").split(/\r?\n/);
  lines.forEach((line, index) => {
    const match = line.match(/^\s*(?:-\s*)?uses:\s*["']?([^\s"'#]+)["']?/);
    if (!match) {
      return;
    }
    const target = match[1];
    if (target.startsWith("./")) {
      return;
    }
    if (target.startsWith("docker://")) {
      if (!dockerDigestPattern.test(target)) {
        failures.push(`${relativePath}:${index + 1}: Docker action must use an sha256 digest: ${target}`);
      }
      return;
    }
    const separator = target.lastIndexOf("@");
    const reference = separator >= 0 ? target.slice(separator + 1) : "";
    if (!commitPattern.test(reference)) {
      failures.push(`${relativePath}:${index + 1}: remote action must use a full commit SHA: ${target}`);
    }
  });
}

if (failures.length > 0) {
  console.error("workflow pin check failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exitCode = 1;
} else {
  console.log("GitHub Actions are pinned to immutable commits");
}
