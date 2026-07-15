const assert = require("node:assert/strict");
const crypto = require("node:crypto");
const { EventEmitter } = require("node:events");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const zlib = require("node:zlib");

const {
  download,
  executable,
  extractBinary,
  installBinary,
  verifyChecksum,
} = require("./install.js");

function scriptedGet(scenarios) {
  const visited = [];
  const get = (url, callback) => {
    visited.push(url.href);
    const scenario = scenarios.shift();
    if (!scenario) throw new Error("unexpected request");
    const request = new EventEmitter();
    request.setTimeout = () => request;
    request.destroy = (error) => process.nextTick(() => request.emit("error", error));
    process.nextTick(() => {
      const response = new EventEmitter();
      response.statusCode = scenario.statusCode ?? 200;
      response.headers = scenario.headers || {};
      response.resume = () => {};
      response.destroy = () => process.nextTick(() => response.emit("aborted"));
      callback(response);
      for (const chunk of scenario.chunks || []) response.emit("data", Buffer.from(chunk));
      response.emit("end");
    });
    return request;
  };
  return { get, visited };
}

function writeTarString(header, start, length, value) {
  Buffer.from(value).copy(header, start, 0, length);
}

function tarMember(name, content, type = "0", prefix = "") {
  const body = Buffer.from(content);
  const header = Buffer.alloc(512);
  writeTarString(header, 0, 100, name);
  writeTarString(header, 100, 8, "0000755\0");
  writeTarString(header, 108, 8, "0000000\0");
  writeTarString(header, 116, 8, "0000000\0");
  writeTarString(header, 124, 12, body.length.toString(8).padStart(11, "0") + "\0");
  writeTarString(header, 136, 12, "00000000000\0");
  header.fill(32, 148, 156);
  writeTarString(header, 156, 1, type);
  writeTarString(header, 257, 6, "ustar\0");
  writeTarString(header, 263, 2, "00");
  writeTarString(header, 345, 155, prefix);
  const checksum = header.reduce((sum, byte) => sum + byte, 0);
  writeTarString(header, 148, 8, checksum.toString(8).padStart(6, "0") + "\0 ");
  return Buffer.concat([header, body, Buffer.alloc((512 - (body.length % 512)) % 512)]);
}

function archive(...members) {
  return zlib.gzipSync(Buffer.concat([...members, Buffer.alloc(1024)]));
}

test("download enforces declared and streamed byte limits", async () => {
  const declared = scriptedGet([{ headers: { "content-length": "9" }, chunks: ["ignored"] }]);
  await assert.rejects(download("https://example.test/archive", { maxBytes: 8, get: declared.get }), /8-byte limit/);

  const streamed = scriptedGet([{ chunks: ["1234", "56789"] }]);
  await assert.rejects(download("https://example.test/archive", { maxBytes: 8, get: streamed.get }), /8-byte limit/);
});

test("download follows only a bounded HTTPS redirect chain", async () => {
  const success = scriptedGet([
    { statusCode: 302, headers: { location: "/release" } },
    { chunks: ["ok"] },
  ]);
  assert.equal((await download("https://example.test/start", { maxBytes: 8, maxRedirects: 1, get: success.get })).toString(), "ok");
  assert.deepEqual(success.visited, ["https://example.test/start", "https://example.test/release"]);

  const loop = scriptedGet([{ statusCode: 302, headers: { location: "/again" } }]);
  await assert.rejects(download("https://example.test/start", { maxBytes: 8, maxRedirects: 0, get: loop.get }), /exceeded 0 redirects/);

  const downgrade = scriptedGet([{ statusCode: 302, headers: { location: "http://example.test/file" } }]);
  await assert.rejects(download("https://example.test/start", { maxBytes: 8, get: downgrade.get }), /must use HTTPS/);
});

test("checksum lookup requires one exact asset and a valid SHA-256", () => {
  const payload = Buffer.from("release");
  const digest = crypto.createHash("sha256").update(payload).digest("hex");
  assert.doesNotThrow(() => verifyChecksum(payload, Buffer.from(`${digest}  ${executable}\n`), executable));
  assert.throws(() => verifyChecksum(payload, Buffer.from(`${digest}  prefix-${executable}\n`), executable), /missing/);
  assert.throws(() => verifyChecksum(payload, Buffer.from(`invalid  ${executable}\n`), executable), /invalid/);
  assert.throws(() => verifyChecksum(payload, Buffer.from(`${digest}  ${executable}\n${digest}  ${executable}\n`), executable), /ambiguous/);
});

test("extractBinary accepts one exact regular bounded member", () => {
  const payload = Buffer.from("binary payload");
  assert.deepEqual(extractBinary(archive(tarMember("README.md", "docs"), tarMember(executable, payload))), payload);
  assert.throws(() => extractBinary(archive(tarMember(`nested/${executable}`, payload))), /exact regular member/);
  assert.throws(() => extractBinary(archive(tarMember(executable, payload, "0", "nested"))), /exact regular member/);
  assert.throws(() => extractBinary(archive(tarMember(executable, "target", "2"))), /not a regular/);
  assert.throws(() => extractBinary(archive(tarMember(executable, payload), tarMember(executable, payload))), /multiple/);
  assert.throws(() => extractBinary(archive(tarMember(executable, payload)), { maxBinaryBytes: 4 }), /binary limit/);
});

test("extractBinary bounds decompression before parsing tar members", () => {
  assert.throws(() => extractBinary(archive(tarMember(executable, Buffer.alloc(4096))), { maxTarBytes: 1024 }), /decompression failed or exceeded/);
});

test("installBinary publishes atomically and cleans staging files", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "openknowledge-npm-install-"));
  const destination = path.join(root, executable);
  try {
    fs.writeFileSync(destination, "old");
    installBinary(Buffer.from("new"), { vendorDir: root, target: destination });
    assert.equal(fs.readFileSync(destination, "utf8"), "new");
    assert.deepEqual(fs.readdirSync(root).filter((name) => name.startsWith(".openknowledge.install.")), []);
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});
