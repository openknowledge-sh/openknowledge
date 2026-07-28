package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrationCommandRequiresRuntimeAndKeepsObservationOptIn(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	wiki := filepath.Join(repo, "Wiki")
	if err := os.MkdirAll(wiki, 0o755); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := captureMainOutput(t, func() int {
		return runIntegration([]string{"install", wiki})
	})
	if code != 2 || !strings.Contains(stderr, "requires --runtime") {
		t.Fatalf("missing runtime code=%d stderr=%q", code, stderr)
	}

	stdout, stderr, code := captureMainOutput(t, func() int {
		return runIntegration([]string{"install", wiki, "--runtime", "claude"})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Session observation is off") {
		t.Fatalf("install code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(repo, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("install enabled observation without --observe: %v", err)
	}

	stdout, stderr, code = captureMainOutput(t, func() int {
		return runIntegration([]string{"status", repo})
	})
	if code != 0 || stderr != "" ||
		!strings.Contains(stdout, "Runtime: claude") ||
		!strings.Contains(stdout, "Observation: disabled") {
		t.Fatalf("status code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	stdout, stderr, code = captureMainOutput(t, func() int {
		return runIntegration([]string{"remove", repo})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Removed Open Knowledge integration") {
		t.Fatalf("remove code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestIntegrationCommandInstallsObservationOnlyForSelectedRuntime(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	wiki := filepath.Join(repo, "Wiki")
	if err := os.MkdirAll(wiki, 0o755); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := captureMainOutput(t, func() int {
		return runIntegration([]string{"install", wiki, "--runtime", "opencode", "--observe"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("install code=%d stderr=%q", code, stderr)
	}
	for _, path := range []string{
		".opencode/skills/openknowledge/SKILL.md",
		".opencode/plugins/openknowledge-observer.js",
	} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
	for _, path := range []string{".agents", ".claude", ".codex"} {
		if _, err := os.Stat(filepath.Join(repo, path)); !os.IsNotExist(err) {
			t.Fatalf("OpenCode install created unrelated path %s", path)
		}
	}
}
