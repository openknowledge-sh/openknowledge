import fs from "node:fs";
import process from "node:process";

const source = fs.readFileSync(new URL("../Dockerfile", import.meta.url), "utf8");
const stages = source.split(/^FROM\s+/m).slice(1);
const failures = [];

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
  console.log("Container runtime uses the unprivileged node user");
}
