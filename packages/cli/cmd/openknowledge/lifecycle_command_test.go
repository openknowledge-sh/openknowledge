package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestSetupWithoutMarkdownOffersCopyAndSaveWhenNoAgentIsInstalled(t *testing.T) {
	repository := t.TempDir()
	t.Setenv("OPENKNOWLEDGE_CODEX", filepath.Join(repository, "missing-codex"))
	t.Setenv("OPENKNOWLEDGE_CLAUDE", filepath.Join(repository, "missing-claude"))
	t.Setenv("OPENKNOWLEDGE_OPENCODE", filepath.Join(repository, "missing-opencode"))
	originalProbe := probeCodexExecutable
	originalInput := setupInput
	originalTerminal := setupInputIsTerminal
	t.Cleanup(func() {
		probeCodexExecutable = originalProbe
		setupInput = originalInput
		setupInputIsTerminal = originalTerminal
	})
	probeCodexExecutable = func(_ context.Context, _ string) error { return errors.New("not installed") }
	setupInput = strings.NewReader("\n\n\n\n")
	setupInputIsTerminal = func() bool { return true }

	var stdout, stderr string
	var code int
	withinDirectory(t, repository, func() {
		stdout, stderr, code = captureMainOutput(t, func() int { return runSetup(nil) })
	})
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "1. Copy the task for an agent") || !strings.Contains(stdout, "2. Save the task to a file") || strings.Contains(stdout, "Run Codex") || strings.Contains(stdout, "Recommended") {
		t.Fatalf("unexpected continuation choices:\n%s", stdout)
	}
}

func TestSetupImportCLIPlansAndCreatesManagedCopy(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "Wiki")
	writeMainTestFile(t, source, "README.md", "# Product\n\nDeployment knowledge.\n")
	writeMainTestFile(t, source, "docs/runbook.md", "# Runbook\n\nRollback the deployment.\n")

	stdout, stderr, code := captureMainOutput(t, func() int {
		return runSetup([]string{target, "--from", source, "--mode", "copy", "--include", "docs", "--plan"})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Setup plan") || !strings.Contains(stdout, "Documents:  1") {
		t.Fatalf("plan code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("--plan wrote target: %v", err)
	}

	stdout, stderr, code = captureMainOutput(t, func() int {
		return runSetup([]string{target, "--from", source, "--mode", "copy", "--include", "README.md", "--include", "docs"})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Knowledge base created") || !strings.Contains(stdout, "Search index    READY") || !strings.Contains(stdout, "Optional enrichment: okn review") {
		t.Fatalf("apply code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(target, "imported", "README.md")); err != nil {
		t.Fatal(err)
	}
}

func TestSetupInteractiveRedirectsExistingBundle(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Wiki")
	if _, err := okf.NewProject(okf.NewProjectOptions{Name: "Knowledge", Path: root, SpecVersion: "latest", Rules: []string{"project", "writing"}, SkipSetup: true}); err != nil {
		t.Fatal(err)
	}
	originalInput := setupInput
	originalTerminal := setupInputIsTerminal
	t.Cleanup(func() { setupInput = originalInput; setupInputIsTerminal = originalTerminal })
	setupInput = strings.NewReader("")
	setupInputIsTerminal = func() bool { return true }
	stdout, stderr, code := captureMainOutput(t, func() int { return runSetup([]string{root}) })
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	for _, expected := range []string{"already set up", "okn check", "okn review", "okn upgrade"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("missing %q:\n%s", expected, stdout)
		}
	}
}

func TestGenericMarkdownSearchReportsUnmanaged(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "README.md", "# Deployment\n\nRollback restores the previous release.\n")
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runSearch([]string{root, "rollback", "--format", "json"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var result okf.ContextResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "unmanaged" || len(result.Sources) == 0 {
		t.Fatalf("result=%#v", result)
	}
}

func TestCheckReportsUnmanagedAndGateFails(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "README.md", "# Knowledge\n")
	stdout, stderr, code := captureMainOutput(t, func() int { return runCheck([]string{root}) })
	if code != 0 || stderr != "" || !strings.Contains(stdout, "UNMANAGED") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	_, _, code = captureMainOutput(t, func() int { return runCheck([]string{root, "--gate"}) })
	if code != 1 {
		t.Fatalf("gate code=%d", code)
	}
}

func TestCheckJSONUsesStableMachineContract(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "README.md", "# Knowledge\n")
	stdout, stderr, code := captureMainOutput(t, func() int { return runCheck([]string{root, "--format", "json"}) })
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var report knowledgeCheckReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != okf.MachineSchemaVersion || report.Overall != checkUnmanaged || len(report.Layers) != 2 {
		t.Fatalf("report=%#v", report)
	}
}

func TestCheckReportsPartialBundleAsBlockedStatus(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, okf.ValidationConfigFile, "[release]\noutputs = []\n")
	stdout, stderr, code := captureMainOutput(t, func() int { return runCheck([]string{root}) })
	if code != 1 || stderr != "" || !strings.Contains(stdout, "BLOCKED") || !strings.Contains(stdout, "Structure") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestReviewFacadePrintsPortableTaskWithoutTerminal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Wiki")
	if _, err := okf.NewProject(okf.NewProjectOptions{Name: "Knowledge", Path: root, SpecVersion: "latest", Rules: []string{"project", "writing"}, SkipSetup: true}); err != nil {
		t.Fatal(err)
	}
	originalTerminal := setupInputIsTerminal
	t.Cleanup(func() { setupInputIsTerminal = originalTerminal })
	setupInputIsTerminal = func() bool { return false }
	stdout, stderr, code := captureMainOutput(t, func() int { return runReview([]string{root, "--scope", "full"}) })
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Open Knowledge Content Review") || !strings.Contains(stdout, "Review scope: `full`") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestPublishRejectsUnmanagedAndBuildsViewer(t *testing.T) {
	unmanaged := t.TempDir()
	writeMainTestFile(t, unmanaged, "README.md", "# Knowledge\n")
	_, stderr, code := captureMainOutput(t, func() int { return runPublish([]string{unmanaged, "--target", "viewer"}) })
	if code != 1 || !strings.Contains(stderr, "managed OKF") {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}

	root := filepath.Join(t.TempDir(), "Wiki")
	if _, err := okf.NewProject(okf.NewProjectOptions{Name: "Knowledge", Path: root, SpecVersion: "latest", Rules: []string{"project", "writing"}, SkipSetup: true}); err != nil {
		t.Fatal(err)
	}
	writeMainTestFile(t, root, okf.ValidationConfigFile, "[release]\noutputs = [\"viewer\"]\n\n[rules]\nenabled = [\"project\", \"writing\"]\n")
	out := filepath.Join(t.TempDir(), "site")
	stdout, stderr, code := captureMainOutput(t, func() int { return runPublish([]string{root, "--target", "viewer", "--out", out, "--plan"}) })
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Publication plan") {
		t.Fatalf("plan code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("publish --plan wrote output: %v", err)
	}
	stdout, stderr, code = captureMainOutput(t, func() int { return runPublish([]string{root, "--target", "viewer", "--out", out}) })
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Publication plan") || !strings.Contains(stdout, "Exported HTML") {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(out, "index.html")); err != nil {
		t.Fatal(err)
	}
}

func TestUpgradeCLIPlansAppliesAndReportsIdempotence(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Wiki")
	if _, err := okf.NewProject(okf.NewProjectOptions{Name: "Knowledge", Path: root, SpecVersion: "0.1", Rules: []string{"project", "writing"}, SkipSetup: true}); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := captureMainOutput(t, func() int { return runUpgrade([]string{root, "--to", "0.2", "--plan"}) })
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Upgrade plan") || !strings.Contains(stdout, "Mechanical changes:") {
		t.Fatalf("plan code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	index, err := os.ReadFile(filepath.Join(root, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), `okf_version: "0.1"`) {
		t.Fatal("--plan changed the bundle")
	}
	stdout, stderr, code = captureMainOutput(t, func() int { return runUpgrade([]string{root, "--to", "0.2"}) })
	if code != 0 || stderr != "" || !strings.Contains(stdout, "from OKF 0.1 to OKF 0.2") {
		t.Fatalf("apply code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	stdout, stderr, code = captureMainOutput(t, func() int { return runUpgrade([]string{root, "--to", "0.2"}) })
	if code != 0 || stderr != "" || !strings.Contains(stdout, "already up to date") {
		t.Fatalf("second code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestPublishMCPPlanResolvesOnlyTheSelectedKnowledgeBase(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "Wiki")
	if _, err := okf.NewProject(okf.NewProjectOptions{Name: "Knowledge", Path: root, SpecVersion: "latest", Rules: []string{"project", "writing"}, SkipSetup: true}); err != nil {
		t.Fatal(err)
	}
	writeMainTestFile(t, root, okf.ValidationConfigFile, "[release]\noutputs = [\"mcp\"]\n\n[rules]\nenabled = [\"project\", \"writing\"]\n")
	configPath := filepath.Join(workspace, "runtime.toml")
	writeMainTestFile(t, workspace, "runtime.toml", fmt.Sprintf(`[runtime]
state_dir = "state"

[artifact_store]
type = "filesystem"
path = "artifacts"

[[knowledge_bases]]
id = "wiki"
path = %q
route = "/wiki"
`, root))
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runPublish([]string{root, "--target", "mcp", "--config", configPath, "--plan"})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "MCP") || !strings.Contains(stdout, configPath) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}
