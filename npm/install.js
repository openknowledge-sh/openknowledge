#!/usr/bin/env node

const crypto = require("crypto");
const fs = require("fs");
const https = require("https");
const os = require("os");
const path = require("path");
const zlib = require("zlib");

const pkg = require("./package.json");

const repo = process.env.OPENKNOWLEDGE_REPO || "openknowledge-sh/openknowledge";
const version = process.env.OPENKNOWLEDGE_VERSION || `v${pkg.version}`;
const vendorDir = path.join(__dirname, "vendor");
const executable = process.platform === "win32" ? "openknowledge.exe" : "openknowledge";
const target = path.join(vendorDir, executable);

function platform() {
  switch (process.platform) {
    case "darwin":
      return "darwin";
    case "linux":
      return "linux";
    case "win32":
      return "windows";
    default:
      throw new Error(`unsupported platform: ${process.platform}`);
  }
}

function arch() {
  switch (process.arch) {
    case "x64":
      return "amd64";
    case "arm64":
      return "arm64";
    default:
      throw new Error(`unsupported architecture: ${process.arch}`);
  }
}

function releaseBase() {
  if (version === "latest") {
    return `https://github.com/${repo}/releases/latest/download`;
  }

  const tag = version.startsWith("v") ? version : `v${version}`;
  return `https://github.com/${repo}/releases/download/${tag}`;
}

function download(url) {
  return new Promise((resolve, reject) => {
    https
      .get(url, (response) => {
        if ([301, 302, 303, 307, 308].includes(response.statusCode)) {
          response.resume();
          download(response.headers.location).then(resolve, reject);
          return;
        }

        if (response.statusCode !== 200) {
          response.resume();
          reject(new Error(`download failed: ${url} (${response.statusCode})`));
          return;
        }

        const chunks = [];
        response.on("data", (chunk) => chunks.push(chunk));
        response.on("end", () => resolve(Buffer.concat(chunks)));
      })
      .on("error", reject);
  });
}

function verifyChecksum(archive, checksums, asset) {
  const line = checksums
    .toString("utf8")
    .split(/\r?\n/)
    .find((entry) => entry.trim().endsWith(asset));

  if (!line) {
    throw new Error(`checksum missing for ${asset}`);
  }

  const expected = line.trim().split(/\s+/)[0];
  const actual = crypto.createHash("sha256").update(archive).digest("hex");

  if (actual !== expected) {
    throw new Error(`checksum mismatch for ${asset}`);
  }
}

function readTarString(buffer, start, length) {
  return buffer
    .subarray(start, start + length)
    .toString("utf8")
    .replace(/\0.*$/, "")
    .trim();
}

function extractBinary(archive) {
  const tar = zlib.gunzipSync(archive);

  for (let offset = 0; offset + 512 <= tar.length; ) {
    const name = readTarString(tar, offset, 100);
    if (!name) {
      break;
    }

    const sizeValue = readTarString(tar, offset + 124, 12);
    const size = Number.parseInt(sizeValue || "0", 8);
    const dataStart = offset + 512;
    const dataEnd = dataStart + size;
    const next = dataStart + Math.ceil(size / 512) * 512;
    const base = path.basename(name);

    if (base === executable) {
      fs.mkdirSync(vendorDir, { recursive: true });
      fs.writeFileSync(target, tar.subarray(dataStart, dataEnd), { mode: 0o755 });
      return;
    }

    offset = next;
  }

  throw new Error(`archive did not contain ${executable}`);
}

async function main() {
  const asset = `openknowledge_${platform()}_${arch()}.tar.gz`;
  const base = releaseBase();
  const archive = await download(`${base}/${asset}`);
  const checksums = await download(`${base}/checksums.txt`);

  verifyChecksum(archive, checksums, asset);
  extractBinary(archive);
}

main().catch((error) => {
  console.error(`openknowledge install failed: ${error.message}`);
  process.exit(1);
});
