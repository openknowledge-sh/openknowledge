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

const MAX_ARCHIVE_BYTES = 64 * 1024 * 1024;
const MAX_CHECKSUM_BYTES = 1024 * 1024;
const MAX_TAR_BYTES = 256 * 1024 * 1024;
const MAX_BINARY_BYTES = 128 * 1024 * 1024;
const MAX_REDIRECTS = 5;
const DOWNLOAD_TIMEOUT_MS = 30_000;

function isSourceWorkspace() {
  return (
    fs.existsSync(path.join(__dirname, "..", "..", "go.work")) &&
    fs.existsSync(path.join(__dirname, "..", "cli", "go.mod"))
  );
}

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

function validatedDownloadURL(value) {
  const parsed = value instanceof URL ? value : new URL(value);
  if (parsed.protocol !== "https:") {
    throw new Error("downloads and redirects must use HTTPS");
  }
  if (parsed.username || parsed.password) {
    throw new Error("download URLs must not contain credentials");
  }
  return parsed;
}

function download(url, options = {}) {
  const maxBytes = options.maxBytes;
  const maxRedirects = options.maxRedirects ?? MAX_REDIRECTS;
  const timeoutMs = options.timeoutMs ?? DOWNLOAD_TIMEOUT_MS;
  const get = options.get || https.get;
  if (!Number.isSafeInteger(maxBytes) || maxBytes <= 0) {
    return Promise.reject(new Error("download limit must be a positive integer"));
  }
  if (!Number.isSafeInteger(maxRedirects) || maxRedirects < 0) {
    return Promise.reject(new Error("redirect limit must be a non-negative integer"));
  }

  const fetch = (currentURL, redirectsRemaining) => new Promise((resolve, reject) => {
    let request;
    try {
      request = get(validatedDownloadURL(currentURL), (response) => {
        if ([301, 302, 303, 307, 308].includes(response.statusCode)) {
          response.resume();
          if (redirectsRemaining === 0) {
            reject(new Error(`download exceeded ${maxRedirects} redirects`));
            return;
          }
          if (!response.headers.location) {
            reject(new Error("download redirect did not include a location"));
            return;
          }
          let nextURL;
          try {
            nextURL = validatedDownloadURL(new URL(response.headers.location, currentURL));
          } catch (error) {
            reject(error);
            return;
          }
          fetch(nextURL, redirectsRemaining - 1).then(resolve, reject);
          return;
        }

        if (response.statusCode !== 200) {
          response.resume();
          reject(new Error(`download failed with HTTP ${response.statusCode}`));
          return;
        }

        const rawLength = Array.isArray(response.headers["content-length"])
          ? response.headers["content-length"][0]
          : response.headers["content-length"];
        if (rawLength !== undefined) {
          if (!/^\d+$/.test(rawLength)) {
            response.resume();
            reject(new Error("download returned an invalid Content-Length"));
            return;
          }
          const declaredLength = Number(rawLength);
          if (!Number.isSafeInteger(declaredLength) || declaredLength > maxBytes) {
            response.resume();
            reject(new Error(`download exceeds the ${maxBytes}-byte limit`));
            return;
          }
        }

        const chunks = [];
        let received = 0;
        response.on("data", (chunk) => {
          received += chunk.length;
          if (received > maxBytes) {
            response.destroy();
            reject(new Error(`download exceeds the ${maxBytes}-byte limit`));
            return;
          }
          chunks.push(chunk);
        });
        response.on("end", () => resolve(Buffer.concat(chunks)));
        response.on("aborted", () => reject(new Error("download response was aborted")));
        response.on("error", reject);
      });
    } catch (error) {
      reject(error);
      return;
    }
    request.on("error", reject);
    if (typeof request.setTimeout === "function" && timeoutMs > 0) {
      request.setTimeout(timeoutMs, () => {
        request.destroy(new Error(`download timed out after ${timeoutMs}ms`));
      });
    }
  });

  let initialURL;
  try {
    initialURL = validatedDownloadURL(url);
  } catch (error) {
    return Promise.reject(error);
  }
  return fetch(initialURL, maxRedirects);
}

function verifyChecksum(archive, checksums, asset) {
  const matches = checksums
    .toString("utf8")
    .split(/\r?\n/)
    .map((entry) => entry.trim().split(/\s+/))
    .filter((fields) => fields.length >= 2 && fields[1].replace(/^\*/, "") === asset);

  if (matches.length === 0) {
    throw new Error(`checksum missing for ${asset}`);
  }
  if (matches.length !== 1) {
    throw new Error(`checksum is ambiguous for ${asset}`);
  }

  const expected = matches[0][0].toLowerCase();
  if (!/^[0-9a-f]{64}$/.test(expected)) {
    throw new Error(`checksum is invalid for ${asset}`);
  }
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

function parseTarOctal(buffer, start, length, field) {
  const value = readTarString(buffer, start, length);
  if (!/^[0-7]+$/.test(value)) {
    throw new Error(`archive has an invalid ${field}`);
  }
  const parsed = Number.parseInt(value, 8);
  if (!Number.isSafeInteger(parsed) || parsed < 0) {
    throw new Error(`archive has an invalid ${field}`);
  }
  return parsed;
}

function tarHeaderChecksum(buffer, offset) {
  let checksum = 0;
  for (let index = 0; index < 512; index += 1) {
    checksum += index >= 148 && index < 156 ? 32 : buffer[offset + index];
  }
  return checksum;
}

function isZeroTarBlock(buffer, offset) {
  for (let index = 0; index < 512; index += 1) {
    if (buffer[offset + index] !== 0) return false;
  }
  return true;
}

function extractBinary(archive, options = {}) {
  const maxTarBytes = options.maxTarBytes ?? MAX_TAR_BYTES;
  const maxBinaryBytes = options.maxBinaryBytes ?? MAX_BINARY_BYTES;
  let tar;
  try {
    tar = zlib.gunzipSync(archive, { maxOutputLength: maxTarBytes });
  } catch (error) {
    throw new Error(`archive decompression failed or exceeded the ${maxTarBytes}-byte limit`);
  }

  let binary = null;

  for (let offset = 0; offset + 512 <= tar.length; ) {
    if (isZeroTarBlock(tar, offset)) {
      break;
    }
    const memberName = readTarString(tar, offset, 100);
    if (!memberName) {
      throw new Error("archive has an empty member name");
    }
    const prefix = readTarString(tar, offset + 345, 155);
    const name = prefix ? `${prefix}/${memberName}` : memberName;

    const storedChecksum = parseTarOctal(tar, offset + 148, 8, "header checksum");
    if (storedChecksum !== tarHeaderChecksum(tar, offset)) {
      throw new Error(`archive member ${name} has an invalid header checksum`);
    }
    const size = parseTarOctal(tar, offset + 124, 12, "member size");
    const dataStart = offset + 512;
    const dataEnd = dataStart + size;
    const next = dataStart + Math.ceil(size / 512) * 512;
    if (!Number.isSafeInteger(next) || dataEnd > tar.length || next > tar.length) {
      throw new Error(`archive member ${name} is truncated`);
    }

    if (name === executable) {
      const type = tar[offset + 156];
      if (type !== 0 && type !== 48) {
        throw new Error(`${executable} is not a regular archive member`);
      }
      if (size <= 0 || size > maxBinaryBytes) {
        throw new Error(`${executable} exceeds the ${maxBinaryBytes}-byte binary limit`);
      }
      if (binary !== null) {
        throw new Error(`archive contains multiple ${executable} members`);
      }
      binary = Buffer.from(tar.subarray(dataStart, dataEnd));
    }

    offset = next;
  }

  if (binary === null) {
    throw new Error(`archive did not contain the exact regular member ${executable}`);
  }
  return binary;
}

function installBinary(binary, options = {}) {
  const destinationDir = options.vendorDir || vendorDir;
  const destination = options.target || target;
  fs.mkdirSync(destinationDir, { recursive: true });
  const staging = path.join(destinationDir, `.openknowledge.install.${process.pid}.${crypto.randomBytes(8).toString("hex")}`);
  try {
    fs.writeFileSync(staging, binary, { flag: "wx", mode: 0o755 });
    fs.renameSync(staging, destination);
  } finally {
    try {
      fs.unlinkSync(staging);
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
  }
}

async function main() {
  if (process.env.OPENKNOWLEDGE_SKIP_DOWNLOAD === "1" || isSourceWorkspace()) {
    console.log("openknowledge install: skipping binary download in source workspace");
    return;
  }
  const asset = `openknowledge_${platform()}_${arch()}.tar.gz`;
  const base = releaseBase();
  const archive = await download(`${base}/${asset}`, { maxBytes: MAX_ARCHIVE_BYTES });
  const checksums = await download(`${base}/checksums.txt`, { maxBytes: MAX_CHECKSUM_BYTES });

  verifyChecksum(archive, checksums, asset);
  installBinary(extractBinary(archive));
}

if (require.main === module) {
  main().catch((error) => {
    console.error(`openknowledge install failed: ${error.message}`);
    process.exit(1);
  });
}

module.exports = {
  DOWNLOAD_TIMEOUT_MS,
  MAX_ARCHIVE_BYTES,
  MAX_BINARY_BYTES,
  MAX_CHECKSUM_BYTES,
  MAX_REDIRECTS,
  MAX_TAR_BYTES,
  download,
  executable,
  extractBinary,
  installBinary,
  verifyChecksum,
};
