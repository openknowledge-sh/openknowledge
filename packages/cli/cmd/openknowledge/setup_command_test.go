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
		`--rules "docs"`,
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
	for _, expected := range []string{"Source: `./source`", "Requested outcome: `Explain releases`", "Depth: 2", "### writing", `enabled = ["project", "writing"]`, "okn setup complete"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("source prompt missing %q:\n%s", expected, stdout)
		}
	}
	if strings.Contains(stdout, "Wiki type:") || strings.Contains(stdout, "--type") {
		t.Fatalf("source prompt must not expose types:\n%s", stdout)
	}
}

func TestSetupFromAcceptsExplicitRules(t *testing.T) {
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runSetup([]string{
			"Wiki", "--prompt", "--from", "./source", "--rules", "project,writing,iso-plain-language",
		})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("setup code=%d stderr=%s", code, stderr)
	}
	for _, expected := range []string{
		`enabled = ["project", "writing", "iso-plain-language"]`,
		`--rules "project,writing,iso-plain-language"`,
		"### iso-plain-language",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("source prompt missing %q:\n%s", expected, stdout)
		}
	}
}

func TestSetupAgentUsesRuntimeValueWithoutGit(t *testing.T) {
	repo := t.TempDir()
	wiki := filepath.Join(repo, "Knowledge")
	expectedExecutable := stubCodexResolver(t, "/test/codex")
	originalRun := runAgentProcess
	t.Cleanup(func() { runAgentProcess = originalRun })
	var prompt string
	runAgentProcess = func(_ context.Context, executable string, arguments []string, directory string) error {
		if executable != expectedExecutable || directory != repo {
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

func TestSetupInteractiveDefersOptionalActivationUntilAfterFirstResult(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
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
	// Base knowledge, print task for the current agent, confirm.
	setupInput = strings.NewReader("\n\n\n")
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
		"What do you want working first?",
		"Base knowledge — searchable documentation with minimal setup",
		"How should setup run?",
		"Print a task for my current agent",
		"Selected maintenance rules:",
		"- project: General project knowledge.",
		"- writing: Apply the common editorial rule",
		"Open Knowledge setup plan",
		"First result:   base knowledge",
		"Later options:  agent instructions, observation, CI, and runtime",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("interactive setup missing %q:\n%s", expected, stdout)
		}
	}
	for _, unexpected := range []string{
		"Install Open Knowledge instructions for agents?",
		"Capture possible knowledge gaps after agent sessions?",
		"The user already selected this activation plan",
	} {
		if strings.Contains(stdout, unexpected) {
			t.Fatalf("interactive setup must defer %q:\n%s", unexpected, stdout)
		}
	}
	searchIndex := strings.Index(stdout, "Run one representative query")
	activationIndex := strings.Index(stdout, "ask the user which installed agent harnesses need Open Knowledge instructions")
	if searchIndex < 0 || activationIndex < 0 || activationIndex < searchIndex {
		t.Fatalf("interactive setup must demonstrate search before optional activation:\n%s", stdout)
	}
}

func TestSetupTrustedKnowledgePresetTailorsTheAgentTask(t *testing.T) {
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runSetup([]string{"Wiki", "--prompt", "--use-case", "trusted"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("setup code=%d stderr=%s", code, stderr)
	}
	for _, expected := range []string{
		"trusted knowledge across multiple sources",
		"Preserve provenance, disagreement, lifecycle, and access boundaries",
		"Ask which trust capabilities the user needs after the first result",
		"Do not enable the complete trust stack by default",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("trusted setup missing %q:\n%s", expected, stdout)
		}
	}
}

func TestParseSetupArgsRejectsRemovedAndAmbiguousOptions(t *testing.T) {
	for _, args := range [][]string{
		{"Wiki", "Other"},
		{"Wiki", "--about", "goal"},
		{"Wiki", "--type", "understanding"},
		{"Wiki", "--runtime", "codex"},
		{"Wiki", "--prompt", "--interactive"},
		{"Wiki", "--prompt", "--agent", "codex"},
		{"Wiki", "--model", "gpt-test"},
		{"Wiki", "--use-case", "everything"},
		{"Wiki", "--use-case", "codebase-docs"},
		{"Wiki", "--use-case", "trusted-knowledge"},
	} {
		if _, err := parseSetupArgs(args); err == nil {
			t.Fatalf("expected setup args to fail: %#v", args)
		}
	}
}

func TestParseSetupArgsAcceptsCanonicalUseCases(t *testing.T) {
	for _, useCase := range []string{"base", "trusted", "custom"} {
		options, err := parseSetupArgs([]string{"Wiki", "--use-case", useCase})
		if err != nil || options.useCase != useCase {
			t.Fatalf("use case %q options=%#v err=%v", useCase, options, err)
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
