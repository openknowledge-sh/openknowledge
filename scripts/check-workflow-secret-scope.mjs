import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const workflowDirectory = path.join(root, ".github", "workflows");
const failures = [];

for (const name of fs.readdirSync(workflowDirectory).sort()) {
  if (!name.endsWith(".yml") && !name.endsWith(".yaml")) {
    continue;
  }
  const relativePath = path.posix.join(".github/workflows", name);
  const lines = fs.readFileSync(path.join(workflowDirectory, name), "utf8").split(/\r?\n/);
  const scopes = [];
  lines.forEach((line, index) => {
    if (/^\s*(?:#.*)?$/.test(line)) {
      return;
    }
    const indentation = line.match(/^\s*/)[0].length;
    while (scopes.length > 0 && scopes.at(-1).indentation >= indentation) {
      scopes.pop();
    }
    const insideStep = scopes.some((scope) => scope.type === "step");
    if (/\$\{\{[^}]*\bsecrets\./.test(line) && !insideStep) {
      failures.push(`${relativePath}:${index + 1}: repository secret must be scoped to a specific step`);
    }
    if (/^\s*secrets:\s*inherit\s*(?:#.*)?$/.test(line)) {
      failures.push(`${relativePath}:${index + 1}: blanket reusable-workflow secret forwarding is forbidden`);
    }

    const content = line.trimStart();
    if (content.startsWith("- ")) {
      const parent = scopes.at(-1);
      scopes.push({
        indentation,
        type: parent?.type === "mapping" && parent.key === "steps" ? "step" : "sequence",
      });
      return;
    }
    const mapping = content.match(/^([^:#]+):(?:\s*(?:#.*)?)?$/);
    if (mapping) {
      scopes.push({ indentation, type: "mapping", key: mapping[1].trim() });
    }
  });
}

if (failures.length > 0) {
  console.error("workflow secret scope check failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exitCode = 1;
} else {
  console.log("Workflow secrets are scoped to explicit consuming steps");
}
