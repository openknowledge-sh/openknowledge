package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestSetupSkillGlobalNeedsNoKnowledgeBase(t *testing.T) {
	home := t.TempDir()
	setSetupTestHome(t, home)
	t.Setenv(okf.RegistryFileEnv, filepath.Join(t.TempDir(), "registry.json"))

	stdout, stderr, code := captureMainOutput(t, func() int {
		return runSetupSkill([]string{"--scope", "global", "--harness", "codex"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("skill code=%d stderr=%s\n%s", code, stderr, stdout)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "openknowledge", "SKILL.md")); err != nil {
		t.Fatalf("global skill missing: %v", err)
	}
	entries, err := okf.RegistryEntries()
	if err != nil || len(entries) != 0 {
		t.Fatalf("global skill changed registry: entries=%+v err=%v", entries, err)
	}
	if !strings.Contains(stdout, "Scope:      global") || !strings.Contains(stdout, "Harnesses:  codex") {
		t.Fatalf("unexpected output:\n%s", stdout)
	}
}

func TestSetupSkillProjectAcceptsRegistryKey(t *testing.T) {
	repo, wiki := setupLifecycleRepository(t)
	if _, _, err := okf.ConnectRegistryEntry("team-wiki", wiki, "read", true); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := captureMainOutput(t, func() int {
		return runSetupSkill([]string{"--scope", "project", "--project", "team-wiki", "--harness", "codex"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("skill code=%d stderr=%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(repo, ".agents", "skills", "openknowledge", "SKILL.md")); err != nil {
		t.Fatalf("project skill missing: %v", err)
	}
	entry, connected, err := okf.ResolveRegistryTarget(wiki)
	if err != nil || !connected || entry.Access != "read" {
		t.Fatalf("existing connection changed: entry=%+v connected=%v err=%v", entry, connected, err)
	}
	status, err := integration.Status(repo)
	if err != nil || !status.ProjectSkills || status.Observe {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestSetupSkillProjectSupportsKnowledgeBaseOutsideGit(t *testing.T) {
	wiki := setupLifecycleStandaloneBundle(t)

	_, stderr, code := captureMainOutput(t, func() int {
		return runSetupSkill([]string{"--scope", "project", "--project", wiki, "--harness", "codex"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("skill code=%d stderr=%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(wiki, ".agents", "skills", "openknowledge", "SKILL.md")); err != nil {
		t.Fatalf("project skill missing: %v", err)
	}
}

func TestSetupSkillInteractiveDetectsHarnessWithoutProject(t *testing.T) {
	home := t.TempDir()
	setSetupTestHome(t, home)
	t.Setenv(okf.RegistryFileEnv, filepath.Join(t.TempDir(), "registry.json"))
	codexExecutable := absoluteTestPath(t, "/test/codex")
	t.Setenv("OPENKNOWLEDGE_CODEX", codexExecutable)
	t.Setenv("OPENKNOWLEDGE_CLAUDE", filepath.FromSlash("/missing/claude"))
	t.Setenv("OPENKNOWLEDGE_OPENCODE", filepath.FromSlash("/missing/opencode"))
	originalProbe := probeCodexExecutable
	originalInput := setupInput
	originalTerminal := setupInputIsTerminal
	t.Cleanup(func() {
		probeCodexExecutable = originalProbe
		setupInput = originalInput
		setupInputIsTerminal = originalTerminal
	})
	probeCodexExecutable = func(_ context.Context, candidate string) error {
		if candidate == codexExecutable {
			return nil
		}
		return errors.New("not installed")
	}
	// Global scope, default detected harnesses, confirm.
	setupInput = strings.NewReader("\n\n\n")
	setupInputIsTerminal = func() bool { return true }

	stdout, stderr, code := captureMainOutput(t, func() int { return runSetupSkill(nil) })
	if code != 0 || stderr != "" {
		t.Fatalf("skill code=%d stderr=%s\n%s", code, stderr, stdout)
	}
	for _, expected := range []string{
		"Where should Open Knowledge install the skill?",
		"Install for which agent environments?",
		"Open Knowledge skill plan",
		"Scope:      global",
		"Harnesses:  codex",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("interactive skill missing %q:\n%s", expected, stdout)
		}
	}
}

func TestParseSetupSkillArgsRequiresExplicitNonInteractiveOptions(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"Wiki"},
		{"--scope", "global"},
		{"--scope", "global", "--project", "Wiki", "--harness", "codex"},
		{"--scope", "project", "--harness", "codex"},
		{"--scope", "unknown", "--harness", "codex"},
	} {
		options, err := parseSetupSkillArgs(args)
		if err == nil {
			err = validateSetupSkillOptions(options)
		}
		if err == nil {
			t.Fatalf("expected args to fail: %#v", args)
		}
	}
}
