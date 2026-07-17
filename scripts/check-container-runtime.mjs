import fs from "node:fs";
import process from "node:process";

const source = fs.readFileSync(new URL("../Dockerfile", import.meta.url), "utf8");
const runtimeSource = fs.readFileSync(new URL("../docker/runtime.Dockerfile", import.meta.url), "utf8");
const runtimeCompose = fs.readFileSync(new URL("../deploy/runtime/docker-compose.yml", import.meta.url), "utf8");
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

checkKnowledgeRuntimeImages(runtimeSource, runtimeCompose, goWorkVersion || goModVersion);

if (failures.length > 0) {
  console.error("container runtime check failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exitCode = 1;
} else {
  console.log("Container toolchains are aligned; web, serve, publisher, and per-harness worker runtimes enforce non-root isolation");
}

function checkKnowledgeRuntimeImages(dockerfile, compose, requiredGoVersion) {
  const runtimeStages = dockerfile.split(/^FROM\s+/m).slice(1);
  const builder = runtimeStages.find((stage) => /\sAS\s+cli-builder\s*$/m.test(stage.split("\n", 1)[0]));
  const serve = runtimeStages.find((stage) => /\sAS\s+serve\s*$/m.test(stage.split("\n", 1)[0]));
  const publisher = runtimeStages.find((stage) => /\sAS\s+publisher\s*$/m.test(stage.split("\n", 1)[0]));
  const workerBase = runtimeStages.find((stage) => /\sAS\s+worker-base\s*$/m.test(stage.split("\n", 1)[0]));
  const workerTargets = Object.fromEntries(
    ["codex", "claude", "grok", "opencode"].map((runtime) => [
      runtime,
      runtimeStages.find((stage) => new RegExp(`\\sAS\\s+worker-${runtime}\\s*$`, "m").test(stage.split("\n", 1)[0])),
    ]),
  );
  if (!builder || !serve || !publisher || !workerBase || Object.values(workerTargets).some((stage) => !stage)) {
    failures.push("runtime.Dockerfile must define cli-builder, serve, publisher, worker-base, and codex/claude/grok/opencode worker targets");
    return;
  }
  const runtimeGo = builder.match(/^golang:([0-9]+\.[0-9]+(?:\.[0-9]+)?)(?:-|@|\s)/);
  if (!runtimeGo || (requiredGoVersion && runtimeGo[1] !== requiredGoVersion)) {
    failures.push(`runtime.Dockerfile Go must match repository Go ${requiredGoVersion}`);
  }
  if (!/^gcr\.io\/distroless\/static-debian12:nonroot/m.test(serve)) {
    failures.push("serve target must use the distroless nonroot runtime");
  }
  if (!/^USER\s+nonroot:nonroot\s*$/m.test(serve) || !/ENTRYPOINT \["\/openknowledge", "runtime", "serve"\]/.test(serve)) {
    failures.push("serve target must select nonroot:nonroot and lock its runtime serve entrypoint");
  }
  if (!/CMD \["--config", "env:OPENKNOWLEDGE_RUNTIME_CONFIG"\]/.test(serve)) {
    failures.push("serve target must default to the provider-injected runtime configuration");
  }
  if (/\b(?:apt-get|npm|git|codex)\b/.test(serve)) {
    failures.push("serve target must not install Git, Node/npm, or Codex runtime tooling");
  }
  if (!/^USER\s+openknowledge:openknowledge\s*$/m.test(publisher) || /@openai\/codex|\bnpm\b/.test(publisher)) {
    failures.push("publisher target must be non-root and must not contain the Codex/Node agent runtime");
  }
  if (!/CMD \["--config", "env:OPENKNOWLEDGE_RUNTIME_CONFIG"\]/.test(publisher)) {
    failures.push("publisher target must default to the provider-injected runtime configuration");
  }
  const harnessContracts = {
    codex: { version: "CODEX_VERSION", install: "@openai/codex@${CODEX_VERSION}" },
    claude: { version: "CLAUDE_CODE_VERSION", install: "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" },
    grok: { version: "GROK_VERSION", install: "@xai-official/grok@${GROK_VERSION}" },
    opencode: { version: "OPENCODE_VERSION", install: "opencode-ai@${OPENCODE_VERSION}" },
  };
  for (const [runtime, worker] of Object.entries(workerTargets)) {
    const contract = harnessContracts[runtime];
    const version = worker.match(new RegExp(`^ARG\\s+${contract.version}=([0-9]+\\.[0-9]+\\.[0-9]+)\\s*$`, "m"));
    if (!version || !worker.includes(contract.install)) {
      failures.push(`worker-${runtime} must install its harness through an explicitly pinned ${contract.version} build argument`);
    }
    if (!/^USER\s+openknowledge:openknowledge\s*$/m.test(worker)) {
      failures.push(`worker-${runtime} must run as openknowledge:openknowledge`);
    }
    if (!new RegExp(`ENV OPENKNOWLEDGE_AGENT_RUNTIME=${runtime}`).test(worker)) {
      failures.push(`worker-${runtime} must declare its closed harness identity`);
    }
    if (!worker.includes(`CMD ["--role", "jobs", "--runtime", "${runtime}", "--config", "env:OPENKNOWLEDGE_RUNTIME_CONFIG"]`)) {
      failures.push(`worker-${runtime} must default to its isolated jobs role and provider-injected configuration`);
    }
  }
  for (const required of [
    "artifacts:/artifacts:ro",
    "cap_drop: [\"ALL\"]",
    "no-new-privileges:true",
    "codex_api_key",
    "anthropic_api_key",
    "xai_api_key",
    "opencode_api_key",
    "github_app_key",
  ]) {
    if (!compose.includes(required)) {
      failures.push(`runtime Compose must include ${required}`);
    }
  }
  for (const runtime of ["codex", "claude", "grok", "opencode"]) {
    const workerService = composeService(compose, `worker-${runtime}`);
    if (!workerService) {
      failures.push(`runtime Compose must define worker-${runtime}`);
      continue;
    }
    if (/^\s+ports:/m.test(workerService)) {
      failures.push(`runtime worker-${runtime} service must not publish ports`);
    }
    if (/github_app_key|artifacts:\/artifacts/.test(workerService)) {
      failures.push(`worker-${runtime} must not mount GitHub credentials or the artifact store`);
    }
  }
  const publisherService = composeService(compose, "publisher");
  if (/codex_api_key|anthropic_api_key|xai_api_key|opencode_api_key|target:\s+worker-/.test(publisherService)) {
    failures.push("publisher must not mount model credentials or an agent worker image");
  }
}

function composeService(source, name) {
  const services = source.split(/^services:\s*$/m)[1] || "";
  const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = services.match(new RegExp(`^  ${escaped}:\\s*$([\\s\\S]*?)(?=^  [A-Za-z0-9_-]+:\\s*$|^volumes:\\s*$|^secrets:\\s*$)`, "m"));
  return match?.[1] || "";
}

function declaredGoVersion(contents, name) {
  const match = contents.match(/^go\s+([0-9]+\.[0-9]+(?:\.[0-9]+)?)\s*$/m);
  if (!match) {
    failures.push(`${name} must declare a Go version`);
    return "";
  }
  return match[1];
}
