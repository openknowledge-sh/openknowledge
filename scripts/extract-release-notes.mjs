import fs from "node:fs";

function fail(message) {
  console.error(`release notes failed: ${message}`);
  process.exit(1);
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

const [rawVersion, changelogPath = "Wiki/changelog/cli.md", outputPath] = process.argv.slice(2);
const version = String(rawVersion || "").trim().replace(/^v/, "");
if (!/^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$/.test(version)) {
  fail(`${JSON.stringify(rawVersion || "")} is not a supported release version`);
}

let changelog;
try {
  changelog = fs.readFileSync(changelogPath, "utf8");
} catch (error) {
  fail(`cannot read ${changelogPath}: ${error.message}`);
}

const heading = new RegExp(`^## v${escapeRegExp(version)} — [0-9]{4}-[0-9]{2}-[0-9]{2}\\s*$`, "m");
const match = heading.exec(changelog);
if (!match) {
  fail(`${changelogPath} does not contain a dated v${version} section`);
}

const contentStart = match.index + match[0].length;
const remaining = changelog.slice(contentStart);
const nextHeading = /^##\s+/m.exec(remaining);
const notes = remaining.slice(0, nextHeading ? nextHeading.index : undefined).trim();
if (!notes) {
  fail(`v${version} has no release notes`);
}

const output = `${notes}\n`;
if (outputPath) {
  fs.writeFileSync(outputPath, output, "utf8");
  console.log(`Wrote release notes for v${version} to ${outputPath}`);
} else {
  process.stdout.write(output);
}
