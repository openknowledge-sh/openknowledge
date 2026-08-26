package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func setSetupTestHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func TestSetupCompleteConnectsAndInstallsBothSkillScopes(t *testing.T) {
	repo, wiki := setupLifecycleRepository(t)
	home := t.TempDir()
	setSetupTestHome(t, home)

	stdout, stderr, code := captureMainOutput(t, func() int {
		return runSetupComplete([]string{wiki, "--skill", "both", "--harness", "codex", "--observe", "off"})
	})
	if code != 0 {
		t.Fatalf("complete code=%d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	entry, ok, err := okf.ResolveRegistryTarget(wiki)
	if err != nil || !ok || entry.Access != "write" {
		t.Fatalf("connection=%+v ok=%v err=%v", entry, ok, err)
	}
	for _, path := range []string{
		filepath.Join(repo, ".agents", "skills", "openknowledge", "SKILL.md"),
		filepath.Join(home, ".agents", "skills", "openknowledge", "SKILL.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing skill %s: %v", path, err)
		}
	}
	if !strings.Contains(stdout, "Open Knowledge setup is complete") || !strings.Contains(stdout, "Observation:    disabled") {
		t.Fatalf("unexpected output:\n%s", stdout)
	}

	// Re-running the finalizer keeps one registry entry and one managed block.
	_, stderr, code = captureMainOutput(t, func() int {
		return runSetupComplete([]string{wiki, "--skill", "both", "--harness", "codex"})
	})
	if code != 0 {
		t.Fatalf("idempotent complete code=%d stderr=%s", code, stderr)
	}
	entries, err := okf.RegistryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("registry entries=%+v err=%v", entries, err)
	}
	projectSkill, err := os.ReadFile(filepath.Join(repo, ".agents", "skills", "openknowledge", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(projectSkill), integration.ProjectManagedStart) != 1 {
		t.Fatalf("project skill is not idempotent:\n%s", projectSkill)
	}
}

func TestSetupCompleteSupportsObservationWithoutSkills(t *testing.T) {
	repo, wiki := setupLifecycleRepository(t)
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runSetupComplete([]string{wiki, "--skill", "none", "--harness", "codex", "--observe", "on"})
	})
	if code != 0 {
		t.Fatalf("complete code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(repo, ".agents", "skills", "openknowledge", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("observation-only setup installed a project skill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".codex", "hooks.json")); err != nil {
		t.Fatalf("observation hook missing: %v", err)
	}
	status, err := integration.Status(repo)
	if err != nil || !status.Observe || status.ProjectSkills {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestSetupCompleteSupportsStandaloneGlobalScope(t *testing.T) {
	wiki := setupLifecycleStandaloneBundle(t)
	home := t.TempDir()
	setSetupTestHome(t, home)

	_, stderr, code := captureMainOutput(t, func() int {
		return runSetupComplete([]string{wiki, "--skill", "global", "--harness", "codex"})
	})
	if code != 0 {
		t.Fatalf("complete code=%d stderr=%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "openknowledge", "SKILL.md")); err != nil {
		t.Fatalf("global skill missing: %v", err)
	}
	if _, connected, err := okf.ResolveRegistryTarget(wiki); err != nil || !connected {
		t.Fatalf("standalone bundle was not connected: connected=%v err=%v", connected, err)
	}
}

func TestSetupCompleteKeepsCLIOnlyBundleLocal(t *testing.T) {
	_, wiki := setupLifecycleRepository(t)
	setSetupTestHome(t, t.TempDir())

	stdout, stderr, code := captureMainOutput(t, func() int {
		return runSetupComplete([]string{wiki, "--skill", "none", "--observe", "off"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("complete code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Connection:     local only") {
		t.Fatalf("local-only completion did not explain its scope:\n%s", stdout)
	}
	if _, connected, err := okf.ResolveRegistryTarget(wiki); err != nil || connected {
		t.Fatalf("CLI-only bundle connection=%v err=%v", connected, err)
	}

	status, statusErr, statusCode := captureMainOutput(t, func() int {
		return runSetupStatus([]string{wiki})
	})
	if statusCode != 0 || statusErr != "" || !strings.Contains(status, "Connection:     not connected") {
		t.Fatalf("status code=%d stdout=%s stderr=%s", statusCode, status, statusErr)
	}
}

func TestSetupCompleteSupportsProjectScopeOutsideGit(t *testing.T) {
	wiki := setupLifecycleStandaloneBundle(t)
	home := t.TempDir()
	setSetupTestHome(t, home)

	_, stderr, code := captureMainOutput(t, func() int {
		return runSetupComplete([]string{wiki, "--skill", "both", "--harness", "codex"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("complete code=%d stderr=%s", code, stderr)
	}
	for _, path := range []string{
		filepath.Join(wiki, ".agents", "skills", "openknowledge", "SKILL.md"),
		filepath.Join(home, ".agents", "skills", "openknowledge", "SKILL.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing skill %s: %v", path, err)
		}
	}
	entries, err := okf.RegistryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("registry entries=%+v err=%v", entries, err)
	}
}

func TestSetupCompleteRefusesTemporaryHandoff(t *testing.T) {
	_, wiki := setupLifecycleRepository(t)
	if err := os.WriteFile(filepath.Join(wiki, "SETUP.MD"), []byte("---\ntype: Setup\n---\n\n# Setup\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := captureMainOutput(t, func() int {
		return runSetupComplete([]string{wiki, "--skill", "none"})
	})
	if code != 1 || !strings.Contains(stderr, "remove SETUP.MD") {
		t.Fatalf("complete code=%d stderr=%s", code, stderr)
	}
	entries, err := okf.RegistryEntries()
	if err != nil || len(entries) != 0 {
		t.Fatalf("incomplete setup changed registry: %+v err=%v", entries, err)
	}
}

func TestSetupObserveCanDisableInstalledHooks(t *testing.T) {
	repo, wiki := setupLifecycleRepository(t)
	_, stderr, code := captureMainOutput(t, func() int {
		return runSetupComplete([]string{wiki, "--skill", "project", "--harness", "codex", "--observe", "on"})
	})
	if code != 0 {
		t.Fatalf("complete code=%d stderr=%s", code, stderr)
	}
	_, stderr, code = captureMainOutput(t, func() int {
		return runSetupObserve([]string{"off", repo})
	})
	if code != 0 {
		t.Fatalf("observe off code=%d stderr=%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(repo, ".codex", "hooks.json")); !os.IsNotExist(err) {
		t.Fatalf("Open Knowledge-owned hook file remains: %v", err)
	}
	status, err := integration.Status(repo)
	if err != nil || status.Observe {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestParseSetupCompleteRequiresExplicitScopeAndHarness(t *testing.T) {
	for _, args := range [][]string{
		{"Wiki"},
		{"Wiki", "--skill", "project"},
		{"Wiki", "--skill", "unknown"},
		{"Wiki", "--skill", "none", "--harness", "codex"},
		{"Wiki", "--skill", "none", "--observe", "maybe"},
	} {
		if _, err := parseSetupCompleteArgs(args); err == nil {
			t.Fatalf("expected args to fail: %#v", args)
		}
	}
}

func setupLifecycleRepository(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init")
	wiki := filepath.Join(repo, "Wiki")
	if err := os.MkdirAll(wiki, 0o755); err != nil {
		t.Fatal(err)
	}
	index := "---\ntype: Index\ntitle: Wiki\nokf_bundle_name: wiki\n---\n\n# Wiki\n"
	if err := os.WriteFile(filepath.Join(wiki, "index.md"), []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(okf.RegistryFileEnv, filepath.Join(t.TempDir(), "registry.json"))
	setSetupTestHome(t, t.TempDir())
	return repo, wiki
}

func setupLifecycleStandaloneBundle(t *testing.T) string {
	t.Helper()
	wiki := filepath.Join(t.TempDir(), "Wiki")
	if err := os.MkdirAll(wiki, 0o755); err != nil {
		t.Fatal(err)
	}
	index := "---\ntype: Index\ntitle: Wiki\nokf_bundle_name: wiki\n---\n\n# Wiki\n"
	if err := os.WriteFile(filepath.Join(wiki, "index.md"), []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(okf.RegistryFileEnv, filepath.Join(t.TempDir(), "registry.json"))
	return wiki
}
