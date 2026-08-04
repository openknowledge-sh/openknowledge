package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupPromptModePrintsCompleteTask(t *testing.T) {
	originalTerminal := setupInputIsTerminal
	t.Cleanup(func() { setupInputIsTerminal = originalTerminal })
	setupInputIsTerminal = func() bool { return false }

	stdout, stderr, code := captureMainOutput(t, func() int {
		return runSetup([]string{"Knowledge", "--prompt", "--rules", "docs"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("setup code=%d stderr=%s", code, stderr)
	}
	for _, expected := range []string{
		"This setup guide is meant to be executed",
		"create or update the knowledge base at Knowledge",
		"Selected maintenance rules:",
		"okn validate",
		"okn setup complete",
		"okn search",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("setup prompt missing %q:\n%s", expected, stdout)
		}
	}
	if strings.Contains(stdout, "Wiki type:") || strings.Contains(stdout, "understanding") {
		t.Fatalf("setup prompt must not expose knowledge-base types:\n%s", stdout)
	}
}

func TestSetupFromUsesIntentWithoutTypes(t *testing.T) {
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runSetup([]string{
			"Wiki", "--prompt", "--from", "./source", "--about", "Explain releases", "--depth", "2",
		})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("setup code=%d stderr=%s", code, stderr)
	}
	for _, expected := range []string{"Source: `./source`", "Requested outcome: `Explain releases`", "Depth: 2", "okn setup complete"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("source prompt missing %q:\n%s", expected, stdout)
		}
	}
	if strings.Contains(stdout, "Wiki type:") || strings.Contains(stdout, "--type") {
		t.Fatalf("source prompt must not expose types:\n%s", stdout)
	}
}

func TestSetupAgentUsesRuntimeValue(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	wiki := filepath.Join(repo, "Knowledge")
	stubCodexResolver(t, "/test/codex")
	originalRun := runAgentProcess
	t.Cleanup(func() { runAgentProcess = originalRun })
	var prompt string
	runAgentProcess = func(_ context.Context, executable string, arguments []string, directory string) error {
		if executable != "/test/codex" || directory != repo {
			t.Fatalf("agent executable=%q directory=%q", executable, directory)
		}
		prompt = arguments[len(arguments)-1]
		return nil
	}

	_, stderr, code := captureMainOutput(t, func() int {
		return runSetup([]string{wiki, "--agent", "codex"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("setup code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(prompt, "okn setup complete") || !strings.Contains(prompt, "Knowledge") {
		t.Fatalf("unexpected agent task:\n%s", prompt)
	}
}

func TestSetupInteractivePrintsSelectedActivationPlan(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	t.Setenv("OPENKNOWLEDGE_CODEX", "/test/codex")
	t.Setenv("OPENKNOWLEDGE_CLAUDE", "/missing/claude")
	t.Setenv("OPENKNOWLEDGE_OPENCODE", "/missing/opencode")
	originalProbe := probeCodexExecutable
	originalInput := setupInput
	originalTerminal := setupInputIsTerminal
	t.Cleanup(func() {
		probeCodexExecutable = originalProbe
		setupInput = originalInput
		setupInputIsTerminal = originalTerminal
	})
	probeCodexExecutable = func(_ context.Context, candidate string) error {
		if candidate == "/test/codex" {
			return nil
		}
		return errors.New("not installed")
	}
	// Project wiki, print task, project skill, default Codex harness,
	// observation off, confirm.
	setupInput = strings.NewReader("\n\n2\n2\n\n\n\n")
	setupInputIsTerminal = func() bool { return true }

	var stdout, stderr string
	var code int
	withinDirectory(t, repo, func() {
		stdout, stderr, code = captureMainOutput(t, func() int { return runSetup(nil) })
	})
	if code != 0 || stderr != "" {
		t.Fatalf("setup code=%d stderr=%s\n%s", code, stderr, stdout)
	}
	for _, expected := range []string{
		"What do you want to set up?",
		"How should setup run?",
		"Which maintenance behaviors should future agents follow?",
		"Install Open Knowledge instructions for agents?",
		"None (not recommended)",
		"Open Knowledge setup plan",
		"--skill project --harness codex --observe off",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("interactive setup missing %q:\n%s", expected, stdout)
		}
	}
}

func TestParseSetupArgsRejectsRemovedAndAmbiguousOptions(t *testing.T) {
	for _, args := range [][]string{
		{"Wiki", "Other"},
		{"Wiki", "--rules", "docs", "--from", "."},
		{"Wiki", "--about", "goal"},
		{"Wiki", "--type", "understanding"},
		{"Wiki", "--runtime", "codex"},
		{"Wiki", "--prompt", "--interactive"},
		{"Wiki", "--prompt", "--agent", "codex"},
		{"Wiki", "--model", "gpt-test"},
	} {
		if _, err := parseSetupArgs(args); err == nil {
			t.Fatalf("expected setup args to fail: %#v", args)
		}
	}
}

func TestSetupPreflightReportsRuntimeRecovery(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	t.Setenv(codexExecutableEnv, "")
	originalDiscover := discoverCodexExecutableCandidates
	originalProbe := probeCodexExecutable
	originalRun := runAgentProcess
	t.Cleanup(func() {
		discoverCodexExecutableCandidates = originalDiscover
		probeCodexExecutable = originalProbe
		runAgentProcess = originalRun
	})
	discoverCodexExecutableCandidates = func() []string { return nil }
	probeCodexExecutable = func(_ context.Context, _ string) error { return errors.New("unavailable") }
	runAgentProcess = func(_ context.Context, _ string, _ []string, _ string) error {
		t.Fatal("setup must not launch an unavailable runtime")
		return nil
	}

	_, stderr, code := captureMainOutput(t, func() int {
		return runSetup([]string{filepath.Join(repo, "Wiki"), "--agent", "codex"})
	})
	if code != 1 || !strings.Contains(stderr, "openknowledge agent doctor --runtime codex") {
		t.Fatalf("setup code=%d stderr=%s", code, stderr)
	}
}

func TestSetupReportsAuthenticationRecoveryAfterAgentFailure(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	stubCodexResolver(t, "/test/codex")
	original := runAgentProcess
	t.Cleanup(func() { runAgentProcess = original })
	runAgentProcess = func(_ context.Context, _ string, _ []string, _ string) error {
		return errors.New("authentication required")
	}

	_, stderr, code := captureMainOutput(t, func() int {
		return runSetup([]string{filepath.Join(repo, "Wiki"), "--agent", "codex"})
	})
	if code != 1 || !strings.Contains(stderr, "verify its authentication") {
		t.Fatalf("setup code=%d stderr=%s", code, stderr)
	}
}

func TestSetupPromptDoesNotCreateBundle(t *testing.T) {
	repo := t.TempDir()
	wiki := filepath.Join(repo, "Wiki")
	_, _, code := captureMainOutput(t, func() int {
		return runSetup([]string{wiki, "--prompt"})
	})
	if code != 0 {
		t.Fatalf("setup prompt exited %d", code)
	}
	if _, err := os.Stat(wiki); !os.IsNotExist(err) {
		t.Fatalf("prompt mode created the bundle: %v", err)
	}
}
