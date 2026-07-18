package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/agents"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const (
	deployRuntimeDirectory        = ".openknowledge/runtime"
	deployRuntimeDockerfile       = deployRuntimeDirectory + "/Dockerfile"
	deployRuntimeEntrypoint       = deployRuntimeDirectory + "/entrypoint.sh"
	deployRuntimeConfig           = deployRuntimeDirectory + "/runtime.toml"
	defaultCodexRuntimeVersion    = "0.128.0"
	defaultClaudeRuntimeVersion   = "2.1.212"
	defaultOpenCodeRuntimeVersion = "1.18.3"
)

var deployRuntimePackageVersionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._-]*$`)
var deployRuntimeReleaseVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)

type deployRuntimeScaffoldResult struct {
	SchemaVersion        string            `json:"schemaVersion"`
	RepositoryRoot       string            `json:"repositoryRoot"`
	Dockerfile           string            `json:"dockerfile"`
	Entrypoint           string            `json:"entrypoint"`
	RuntimeConfig        string            `json:"runtimeConfig"`
	OpenKnowledgeVersion string            `json:"openknowledgeVersion"`
	AgentVersions        map[string]string `json:"agentVersions,omitempty"`
}

type deployRuntimeScaffoldOptions struct {
	Runtimes             string
	OpenKnowledgeVersion string
	CodexVersion         string
	ClaudeVersion        string
	OpenCodeVersion      string
	Force                bool
}

func runDeployRailwayInit(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, deployRailwayInitHelpText())
		return 0
	}
	if len(args) > 1 && !strings.HasPrefix(args[0], "-") {
		args = append(append([]string(nil), args[1:]...), args[0])
	}
	flags := flag.NewFlagSet("deploy railway init", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	runtimes := flags.String("runtimes", "", "comma-separated agent runtimes to install")
	openKnowledgeVersion := flags.String("openknowledge-version", version, "Open Knowledge npm package version")
	codexVersion := flags.String("codex-version", defaultCodexRuntimeVersion, "Codex CLI npm package version")
	claudeVersion := flags.String("claude-version", defaultClaudeRuntimeVersion, "Claude Code npm package version")
	openCodeVersion := flags.String("opencode-version", defaultOpenCodeRuntimeVersion, "OpenCode npm package version")
	force := flags.Bool("force", false, "replace an existing generated runtime scaffold")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "deploy railway init accepts at most one knowledge base path")
		return 2
	}
	knowledgePath := "."
	if flags.NArg() == 1 {
		knowledgePath = flags.Arg(0)
	}
	result, err := scaffoldRailwayRuntime(knowledgePath, deployRuntimeScaffoldOptions{
		Runtimes: *runtimes, OpenKnowledgeVersion: *openKnowledgeVersion,
		CodexVersion: *codexVersion, ClaudeVersion: *claudeVersion,
		OpenCodeVersion: *openCodeVersion, Force: *force,
	})
	if err != nil {
		return printAgentCommandError(err)
	}
	if err := printJSON(result); err != nil {
		return printAgentCommandError(err)
	}
	return 0
}

func scaffoldRailwayRuntime(knowledgeInput string, options deployRuntimeScaffoldOptions) (deployRuntimeScaffoldResult, error) {
	root, err := okf.ResolveKnowledgeRoot(knowledgeInput)
	if err != nil {
		return deployRuntimeScaffoldResult{}, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return deployRuntimeScaffoldResult{}, err
	}
	repoRoot, err := runtimeGitOutput(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return deployRuntimeScaffoldResult{}, fmt.Errorf("knowledge base must be inside a Git repository: %w", err)
	}
	if evaluated, evalErr := filepath.EvalSymlinks(root); evalErr == nil {
		root = evaluated
	}
	if evaluated, evalErr := filepath.EvalSymlinks(repoRoot); evalErr == nil {
		repoRoot = evaluated
	}
	relative, err := filepath.Rel(repoRoot, root)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return deployRuntimeScaffoldResult{}, fmt.Errorf("knowledge base path must be inside the Git repository")
	}
	knowledgePath := "/workspace"
	if relative != "." {
		knowledgePath += "/" + filepath.ToSlash(relative)
	}
	knowledgeID := sanitizeDeployName(filepath.Base(root))
	if knowledgeID == "" {
		knowledgeID = "knowledge"
	}
	runtimes, err := resolveDeployAgentRuntimes(repoRoot, options.Runtimes, false)
	if err != nil {
		return deployRuntimeScaffoldResult{}, err
	}
	versions := map[string]string{
		agents.RuntimeCodex:    strings.TrimSpace(options.CodexVersion),
		agents.RuntimeClaude:   strings.TrimSpace(options.ClaudeVersion),
		agents.RuntimeOpenCode: strings.TrimSpace(options.OpenCodeVersion),
	}
	openKnowledgeVersion := strings.TrimSpace(options.OpenKnowledgeVersion)
	if !deployRuntimeReleaseVersionPattern.MatchString(openKnowledgeVersion) {
		return deployRuntimeScaffoldResult{}, fmt.Errorf("Open Knowledge release version is invalid: %q", openKnowledgeVersion)
	}
	for _, runtimeName := range runtimes {
		if err := validateDeployRuntimePackageVersion(runtimeName, versions[runtimeName]); err != nil {
			return deployRuntimeScaffoldResult{}, err
		}
	}
	dockerfile := renderDeployRuntimeDockerfile(openKnowledgeVersion, runtimes, versions)
	entrypoint := renderDeployRuntimeEntrypoint()
	runtimeConfig := renderDeployRuntimeConfig(knowledgeID, knowledgePath)
	dockerfilePath := filepath.Join(repoRoot, filepath.FromSlash(deployRuntimeDockerfile))
	entrypointPath := filepath.Join(repoRoot, filepath.FromSlash(deployRuntimeEntrypoint))
	runtimeConfigPath := filepath.Join(repoRoot, filepath.FromSlash(deployRuntimeConfig))
	if err := writeDeployRuntimeScaffoldFile(dockerfilePath, []byte(dockerfile), 0o644, options.Force); err != nil {
		return deployRuntimeScaffoldResult{}, err
	}
	if err := writeDeployRuntimeScaffoldFile(entrypointPath, []byte(entrypoint), 0o755, options.Force); err != nil {
		return deployRuntimeScaffoldResult{}, err
	}
	if err := writeDeployRuntimeScaffoldFile(runtimeConfigPath, []byte(runtimeConfig), 0o644, options.Force); err != nil {
		return deployRuntimeScaffoldResult{}, err
	}
	agentVersions := make(map[string]string, len(runtimes))
	for _, runtimeName := range runtimes {
		agentVersions[runtimeName] = versions[runtimeName]
	}
	return deployRuntimeScaffoldResult{
		SchemaVersion: okf.MachineSchemaVersion, RepositoryRoot: repoRoot,
		Dockerfile: dockerfilePath, Entrypoint: entrypointPath, RuntimeConfig: runtimeConfigPath,
		OpenKnowledgeVersion: openKnowledgeVersion, AgentVersions: agentVersions,
	}, nil
}

func validateDeployRuntimePackageVersion(name string, value string) error {
	if !deployRuntimePackageVersionPattern.MatchString(value) {
		return fmt.Errorf("%s package version is invalid: %q", name, value)
	}
	return nil
}

func writeDeployRuntimeScaffoldFile(path string, content []byte, mode os.FileMode, force bool) error {
	existing, err := os.ReadFile(path)
	if err == nil {
		if string(existing) == string(content) {
			return os.Chmod(path, mode)
		}
		if !force {
			return fmt.Errorf("refusing to replace %s; review it or rerun with --force", path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := writeOutputFileAtomically(path, content); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

func renderDeployRuntimeDockerfile(openKnowledgeVersion string, runtimes []string, versions map[string]string) string {
	runtimes = append([]string(nil), runtimes...)
	sort.Strings(runtimes)
	var arguments strings.Builder
	var packages []string
	for _, runtimeName := range runtimes {
		switch runtimeName {
		case agents.RuntimeCodex:
			fmt.Fprintf(&arguments, "ARG CODEX_VERSION=%s\n", versions[runtimeName])
			packages = append(packages, `"@openai/codex@${CODEX_VERSION}"`)
		case agents.RuntimeClaude:
			fmt.Fprintf(&arguments, "ARG CLAUDE_CODE_VERSION=%s\n", versions[runtimeName])
			packages = append(packages, `"@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}"`)
		case agents.RuntimeOpenCode:
			fmt.Fprintf(&arguments, "ARG OPENCODE_VERSION=%s\n", versions[runtimeName])
			packages = append(packages, `"opencode-ai@${OPENCODE_VERSION}"`)
		}
	}
	install := ""
	if len(packages) > 0 {
		separator := " \\" + "\n    "
		install = "RUN npm install --global " + strings.Join(packages, separator) + separator + "&& npm cache clean --force"
	}
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.7

FROM node:22-bookworm-slim AS runtime

ARG OPENKNOWLEDGE_VERSION=%s
ARG TARGETOS
ARG TARGETARCH
%s
RUN apt-get update \
    && apt-get install --no-install-recommends -y ca-certificates curl git gosu tini \
    && rm -rf /var/lib/apt/lists/*
RUN set -eux; \
    asset="openknowledge_${TARGETOS}_${TARGETARCH}.tar.gz"; \
    base="https://github.com/openknowledge-sh/openknowledge/releases/download/v${OPENKNOWLEDGE_VERSION}"; \
    curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 "$base/$asset" -o "/tmp/$asset"; \
    curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 "$base/checksums.txt" -o /tmp/checksums.txt; \
    cd /tmp; \
    grep "  $asset$" checksums.txt | sha256sum -c -; \
    tar -xzf "$asset" -C /usr/local/bin openknowledge; \
    chmod 0755 /usr/local/bin/openknowledge; \
    rm -f "$asset" checksums.txt
%s

RUN groupadd --system --gid 10001 openknowledge \
    && useradd --system --uid 10001 --gid openknowledge --home-dir /var/lib/openknowledge --create-home openknowledge \
    && mkdir -p /var/lib/openknowledge /workspace /opt/openknowledge/artifacts /etc/openknowledge \
    && chown -R openknowledge:openknowledge /var/lib/openknowledge /workspace /opt/openknowledge

FROM runtime AS build
WORKDIR /workspace
COPY . /workspace
ARG RAILWAY_GIT_COMMIT_SHA=local
RUN openknowledge runtime build \
    --config /workspace/.openknowledge/runtime/runtime.toml \
    --commit "${RAILWAY_GIT_COMMIT_SHA:-local}"

FROM runtime

COPY .openknowledge/runtime/entrypoint.sh /usr/local/bin/openknowledge-runtime-entrypoint
COPY .openknowledge/runtime/runtime.toml /etc/openknowledge/runtime.toml
COPY --from=build /opt/openknowledge/artifacts /opt/openknowledge/artifacts
RUN chmod 0755 /usr/local/bin/openknowledge-runtime-entrypoint \
    && chown -R openknowledge:openknowledge /opt/openknowledge/artifacts /etc/openknowledge/runtime.toml

USER root:root
ENV OPENKNOWLEDGE_ROLE=serve
EXPOSE 8080
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/openknowledge-runtime-entrypoint"]
`, openKnowledgeVersion, arguments.String(), install)
}

func renderDeployRuntimeConfig(knowledgeID string, knowledgePath string) string {
	return fmt.Sprintf(`[runtime]
state_dir = "/tmp/openknowledge"

[artifact_store]
type = "filesystem"
path = "/opt/openknowledge/artifacts"

[serve]
address = "0.0.0.0:8080"
poll_interval = "5s"
request_timeout = "30s"
max_concurrency = 64
mcp_access = "public"
mcp_token_env = "OPENKNOWLEDGE_MCP_TOKEN"

[worker]
repo = "/workspace"
remote = "origin"
production_branch = "main"
poll_interval = "30s"
run_jobs = false
jobs_path = ".openknowledge/jobs"
exchange_dir = "/tmp/openknowledge/exchange"

[[knowledge_bases]]
id = %q
path = %q
route = "/"
spec = "latest"
publish = true
mcp = true
`, knowledgeID, knowledgePath)
}

func renderDeployRuntimeEntrypoint() string {
	return `#!/bin/sh
set -eu

config="${OPENKNOWLEDGE_RUNTIME_CONFIG_PATH:-}"
if [ -z "$config" ]; then
  if [ -n "${OPENKNOWLEDGE_RUNTIME_CONFIG:-}" ]; then
    config="env:OPENKNOWLEDGE_RUNTIME_CONFIG"
  else
    config="/etc/openknowledge/runtime.toml"
  fi
fi

run_as_openknowledge() {
  if [ "$(id -u)" -eq 0 ]; then
    chown -R openknowledge:openknowledge /var/lib/openknowledge
    exec gosu openknowledge:openknowledge "$@"
  fi
  exec "$@"
}

case "${OPENKNOWLEDGE_ROLE:-serve}" in
  serve)
    run_as_openknowledge openknowledge runtime serve --config "$config"
    ;;
  publisher)
    run_as_openknowledge openknowledge runtime worker --role publisher --config "$config"
    ;;
  worker)
    runtime="${OPENKNOWLEDGE_AGENT_RUNTIME:?OPENKNOWLEDGE_AGENT_RUNTIME is required for the worker role}"
    run_as_openknowledge openknowledge runtime worker --role jobs --runtime "$runtime" --config "$config"
    ;;
  *)
    echo "OPENKNOWLEDGE_ROLE must be serve, publisher, or worker" >&2
    exit 2
    ;;
esac
`
}

func deployRailwayInitHelpText() string {
	return `openknowledge deploy railway init [path] [options]

Create a repository-owned Railway runtime Dockerfile, immutable build config,
and entrypoint. The image builds the knowledge artifact from the source commit
and serves it by default. Package versions stay under project control. Existing
files are never replaced unless --force is passed.

Options:
  --runtimes LIST                  Agent runtimes to install (none by default).
  --openknowledge-version VERSION Open Knowledge GitHub release (default: this CLI).
  --codex-version VERSION         Codex CLI package version.
  --claude-version VERSION        Claude Code package version.
  --opencode-version VERSION      OpenCode package version.
  --force                         Replace an existing generated scaffold.
`
}
