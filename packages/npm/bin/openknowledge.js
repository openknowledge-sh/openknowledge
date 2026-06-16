#!/usr/bin/env node

const fs = require("fs");
const path = require("path");
const { spawn } = require("child_process");

const executable = process.platform === "win32" ? "openknowledge.exe" : "openknowledge";
const binary = path.join(__dirname, "..", "vendor", executable);

if (!fs.existsSync(binary)) {
  console.error("openknowledge binary is missing. Reinstall @openknowledge-sh/openknowledge.");
  process.exit(1);
}

const child = spawn(binary, process.argv.slice(2), { stdio: "inherit" });

child.on("error", (error) => {
  console.error(error.message);
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 1);
});
