package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRailwayRuntimeScaffoldPinsProjectOwnedPackages(t *testing.T) {
	repository, wiki := newDeployTestRepository(t)
	removeDeployRuntimeScaffoldForTest(t, repository)

	result, err := scaffoldRailwayRuntime(wiki, deployRuntimeScaffoldOptions{
		Runtimes: "codex", OpenKnowledgeVersion: "0.7.0", CodexVersion: "0.129.1",
		ClaudeVersion: defaultClaudeRuntimeVersion, OpenCodeVersion: defaultOpenCodeRuntimeVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	dockerfile, err := os.ReadFile(result.Dockerfile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(dockerfile)
	for _, expected := range []string{
		"ARG OPENKNOWLEDGE_VERSION=0.7.0",
		"ARG CODEX_VERSION=0.129.1",
		`releases/download/v${OPENKNOWLEDGE_VERSION}`,
		`grep "  $asset$" checksums.txt | sha256sum -c -`,
		`"@openai/codex@${CODEX_VERSION}"`,
		"ca-certificates curl git gosu tini",
		"COPY .openknowledge/runtime/entrypoint.sh",
		"USER root:root",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("generated Dockerfile is missing %q:\n%s", expected, content)
		}
	}
	if strings.Contains(content, "ghcr.io/openknowledge-sh") {
		t.Fatalf("generated runtime unexpectedly depends on a published Open Knowledge image:\n%s", content)
	}
	if strings.Contains(content, "\n+") {
		t.Fatalf("generated Dockerfile contains patch markers:\n%s", content)
	}
	entrypoint, err := os.ReadFile(result.Entrypoint)
	if err != nil {
		t.Fatal(err)
	}
	entrypointContent := string(entrypoint)
	for _, role := range []string{"serve)", "publisher)", "worker)"} {
		if !strings.Contains(entrypointContent, role) {
			t.Fatalf("entrypoint is missing %s role:\n%s", role, entrypoint)
		}
	}
	for _, expected := range []string{
		"chown -R openknowledge:openknowledge /var/lib/openknowledge",
		`exec gosu openknowledge:openknowledge "$@"`,
	} {
		if !strings.Contains(entrypointContent, expected) {
			t.Fatalf("entrypoint is missing %q:\n%s", expected, entrypoint)
		}
	}
	info, err := os.Stat(result.Entrypoint)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("entrypoint mode = %o, want 755", info.Mode().Perm())
	}
}

func TestRailwayRuntimeScaffoldDoesNotOverwriteProjectChoicesWithoutForce(t *testing.T) {
	repository, wiki := newDeployTestRepository(t)
	removeDeployRuntimeScaffoldForTest(t, repository)
	options := deployRuntimeScaffoldOptions{
		Runtimes: "codex", OpenKnowledgeVersion: "0.7.0", CodexVersion: "0.129.1",
		ClaudeVersion: defaultClaudeRuntimeVersion, OpenCodeVersion: defaultOpenCodeRuntimeVersion,
	}
	if _, err := scaffoldRailwayRuntime(wiki, options); err != nil {
		t.Fatal(err)
	}
	options.CodexVersion = "0.130.0"
	if _, err := scaffoldRailwayRuntime(wiki, options); err == nil || !strings.Contains(err.Error(), "refusing to replace") {
		t.Fatalf("expected overwrite refusal, got %v", err)
	}
	options.Force = true
	if _, err := scaffoldRailwayRuntime(wiki, options); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(repository, filepath.FromSlash(deployRuntimeDockerfile)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "ARG CODEX_VERSION=0.130.0") {
		t.Fatalf("forced scaffold did not update the project pin:\n%s", content)
	}
}

func removeDeployRuntimeScaffoldForTest(t *testing.T, repository string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(repository, filepath.FromSlash(deployRuntimeDirectory))); err != nil {
		t.Fatal(err)
	}
}
