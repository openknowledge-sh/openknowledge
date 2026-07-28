import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const packageDirectory = path.join(root, "packages", "npm");
const temporaryDirectory = fs.mkdtempSync(path.join(os.tmpdir(), "openknowledge-packed-npm-"));
const installDirectory = path.join(temporaryDirectory, "consumer");
const npmEnvironment = {
  OPENKNOWLEDGE_SKIP_DOWNLOAD: "1",
  npm_config_cache: path.join(temporaryDirectory, "npm-cache"),
};

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: options.cwd || root,
    encoding: "utf8",
    env: { ...process.env, ...options.env },
    shell: process.platform === "win32",
  });
  if (result.status !== 0) {
    throw new Error(
      `${command} ${args.join(" ")} failed\n${result.stdout || ""}${result.stderr || ""}`,
    );
  }
  return result.stdout;
}

try {
  const packOutput = run(
    "npm",
    ["pack", "--json", "--pack-destination", temporaryDirectory],
    { cwd: packageDirectory, env: npmEnvironment },
  );
  const packResult = JSON.parse(packOutput);
  assert.equal(packResult.length, 1, "npm pack must produce exactly one archive");
  const archive = path.join(temporaryDirectory, packResult[0].filename);
  assert.ok(fs.statSync(archive).isFile(), "npm pack archive must exist");

  fs.mkdirSync(installDirectory);
  fs.writeFileSync(
    path.join(installDirectory, "package.json"),
    JSON.stringify({ name: "openknowledge-install-smoke", private: true }),
  );
  run("npm", ["install", "--no-audit", "--no-fund", archive], {
    cwd: installDirectory,
    env: npmEnvironment,
  });

  const installedManifest = JSON.parse(
    fs.readFileSync(
      path.join(installDirectory, "node_modules", "@openknowledge-sh", "openknowledge", "package.json"),
      "utf8",
    ),
  );
  assert.deepEqual(installedManifest.bin, {
    openknowledge: "bin/openknowledge.js",
    okn: "bin/openknowledge.js",
  });

  const binDirectory = path.join(installDirectory, "node_modules", ".bin");
  const suffix = process.platform === "win32" ? ".cmd" : "";
  for (const command of ["openknowledge", "okn"]) {
    assert.ok(
      fs.existsSync(path.join(binDirectory, command + suffix)),
      `packed install must expose ${command}`,
    );
  }

  console.log(`Packed npm install smoke test passed on Node ${process.versions.node}`);
} finally {
  fs.rmSync(temporaryDirectory, { recursive: true, force: true });
}
