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
	runtimes := flags.String("runtimes", "", "comma-separated agent runtimes; inferred from enabled jobs when omitted")
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
	repoRoot, err := runtimeGitOutput(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return deployRuntimeScaffoldResult{}, fmt.Errorf("knowledge base must be inside a Git repository: %w", err)
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
	dockerfilePath := filepath.Join(repoRoot, filepath.FromSlash(deployRuntimeDockerfile))
	entrypointPath := filepath.Join(repoRoot, filepath.FromSlash(deployRuntimeEntrypoint))
	if err := writeDeployRuntimeScaffoldFile(dockerfilePath, []byte(dockerfile), 0o644, options.Force); err != nil {
		return deployRuntimeScaffoldResult{}, err
	}
	if err := writeDeployRuntimeScaffoldFile(entrypointPath, []byte(entrypoint), 0o755, options.Force); err != nil {
		return deployRuntimeScaffoldResult{}, err
	}
	agentVersions := make(map[string]string, len(runtimes))
	for _, runtimeName := range runtimes {
		agentVersions[runtimeName] = versions[runtimeName]
	}
	return deployRuntimeScaffoldResult{
		SchemaVersion: okf.MachineSchemaVersion, RepositoryRoot: repoRoot,
		Dockerfile: dockerfilePath, Entrypoint: entrypointPath,
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

FROM node:22-bookworm-slim

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
    && mkdir -p /var/lib/openknowledge /workspace /artifacts /exchange \
    && chown -R openknowledge:openknowledge /var/lib/openknowledge /workspace /artifacts /exchange

COPY .openknowledge/runtime/entrypoint.sh /usr/local/bin/openknowledge-runtime-entrypoint
RUN chmod 0755 /usr/local/bin/openknowledge-runtime-entrypoint

USER root:root
EXPOSE 8080
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/openknowledge-runtime-entrypoint"]
`, openKnowledgeVersion, arguments.String(), install)
}

func renderDeployRuntimeEntrypoint() string {
	return `#!/bin/sh
set -eu

config="${OPENKNOWLEDGE_RUNTIME_CONFIG_PATH:-env:OPENKNOWLEDGE_RUNTIME_CONFIG}"

run_as_openknowledge() {
  if [ "$(id -u)" -eq 0 ]; then
    chown -R openknowledge:openknowledge /var/lib/openknowledge
    exec gosu openknowledge:openknowledge "$@"
  fi
  exec "$@"
}

case "${OPENKNOWLEDGE_ROLE:-}" in
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

Create a repository-owned Railway runtime Dockerfile and entrypoint. Package
versions are pinned in the generated Dockerfile and remain under project
control. Existing files are never replaced unless --force is passed.

Options:
  --runtimes LIST                  Agent runtimes; infer enabled jobs when omitted.
  --openknowledge-version VERSION Open Knowledge GitHub release (default: this CLI).
  --codex-version VERSION         Codex CLI package version.
  --claude-version VERSION        Claude Code package version.
  --opencode-version VERSION      OpenCode package version.
  --force                         Replace an existing generated scaffold.
`
}
