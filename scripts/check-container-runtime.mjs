import fs from "node:fs";
import process from "node:process";

const source = fs.readFileSync(new URL("../Dockerfile", import.meta.url), "utf8");
const goMod = fs.readFileSync(new URL("../packages/cli/go.mod", import.meta.url), "utf8");
const goWork = fs.readFileSync(new URL("../go.work", import.meta.url), "utf8");
const stages = source.split(/^FROM\s+/m).slice(1);
const failures = [];

const goModVersion = declaredGoVersion(goMod, "packages/cli/go.mod");
const goWorkVersion = declaredGoVersion(goWork, "go.work");
if (goModVersion && goWorkVersion && goModVersion !== goWorkVersion) {
  failures.push(`Go module and workspace versions differ: ${goModVersion} != ${goWorkVersion}`);
}

const buildImage = stages[0]?.match(/^golang:([0-9]+\.[0-9]+(?:\.[0-9]+)?)(?:-|@|\s)/);
if (!buildImage) {
  failures.push("first Dockerfile stage must use an explicitly versioned golang image");
} else {
  const requiredGoVersion = goWorkVersion || goModVersion;
  if (requiredGoVersion && buildImage[1] !== requiredGoVersion) {
    failures.push(`Dockerfile Go ${buildImage[1]} must match repository Go ${requiredGoVersion}`);
  }
}

if (stages.length < 2) {
  failures.push("Dockerfile must use a separate runtime stage");
} else {
  const runtime = stages.at(-1);
  const users = [...runtime.matchAll(/^USER\s+([^\s#]+).*$/gm)].map((match) => match[1]);
  if (users.length === 0) {
    failures.push("final container stage must declare a non-root USER");
  } else if (users.at(-1) === "root" || users.at(-1) === "0") {
    failures.push(`final container stage ends as privileged user ${users.at(-1)}`);
  } else if (users.at(-1) !== "node") {
    failures.push(`final Node container stage must use the image's node user, got ${users.at(-1)}`);
  }

  const userIndex = runtime.lastIndexOf(`USER ${users.at(-1) || ""}`);
  const commandIndex = Math.max(runtime.lastIndexOf("\nCMD "), runtime.lastIndexOf("\nENTRYPOINT "));
  if (userIndex >= 0 && commandIndex >= 0 && userIndex > commandIndex) {
    failures.push("runtime USER must be selected before CMD or ENTRYPOINT");
  }
}

if (failures.length > 0) {
  console.error("container runtime check failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exitCode = 1;
} else {
  console.log("Container Go toolchain is aligned and runtime uses the unprivileged node user");
}

function declaredGoVersion(contents, name) {
  const match = contents.match(/^go\s+([0-9]+\.[0-9]+(?:\.[0-9]+)?)\s*$/m);
  if (!match) {
    failures.push(`${name} must declare a Go version`);
    return "";
  }
  return match[1];
}
