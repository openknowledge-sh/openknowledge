package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupRunsAgentValidatesAndIntegrates(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	wiki := filepath.Join(repo, "Knowledge")
	stubCodexResolver(t, "/test/codex")
	original := runAgentProcess
	t.Cleanup(func() { runAgentProcess = original })
	var prompt string
	runAgentProcess = func(_ context.Context, _ string, arguments []string, directory string) error {
		prompt = arguments[len(arguments)-1]
		if directory != repo {
			t.Fatalf("agent directory=%q want %q", directory, repo)
		}
		if err := os.MkdirAll(wiki, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(wiki, "index.md"), []byte("---\ntype: Index\n---\n\n# Knowledge\n"), 0o644)
	}

	stdout, stderr, code := captureMainOutput(t, func() int {
		return runSetup([]string{wiki, "--rules", "docs", "--agent", "--runtime", "codex"})
	})
	if code != 0 {
		t.Fatalf("setup code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Ready: Knowledge") || strings.Contains(stdout, "Next:") {
		t.Fatalf("setup should finish without prescribing another onboarding command:\n%s", stdout)
	}
	if !strings.Contains(prompt, "This setup guide is meant to be executed") || !strings.Contains(prompt, "Knowledge") || !strings.Contains(prompt, "Selected maintenance rules") {
		t.Fatalf("unexpected setup prompt:\n%s", prompt)
	}
	for _, path := range []string{".openknowledge/integration.toml", ".agents/skills/openknowledge/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing integration file %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".codex", "hooks.json")); !os.IsNotExist(err) {
		t.Fatalf("setup must not enable session observation by default: %v", err)
	}
}

func TestSetupFromUsesSourceWorkflowAndTarget(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	wiki := filepath.Join(repo, "Wiki")
	stubCodexResolver(t, "/test/codex")
	original := runAgentProcess
	t.Cleanup(func() { runAgentProcess = original })
	var prompt string
	runAgentProcess = func(_ context.Context, _ string, arguments []string, _ string) error {
		prompt = arguments[len(arguments)-1]
		if err := os.MkdirAll(wiki, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(wiki, "index.md"), []byte("---\ntype: Index\n---\n\n# Wiki\n"), 0o644)
	}

	_, stderr, code := captureMainOutput(t, func() int {
		return runSetup([]string{wiki, "--from", ".", "--type", "custom", "--about", "Explain releases", "--agent", "--runtime", "codex"})
	})
	if code != 0 {
		t.Fatalf("setup --from code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(prompt, "Source: `.`") || !strings.Contains(prompt, "Output wiki path: `Wiki`") || !strings.Contains(prompt, "Explain releases") {
		t.Fatalf("unexpected source workflow prompt:\n%s", prompt)
	}
}

func TestSetupWithoutArgumentsPrintsOpenEndedPrompt(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	original := runAgentProcess
	t.Cleanup(func() { runAgentProcess = original })
	runAgentProcess = func(_ context.Context, _ string, _ []string, _ string) error {
		t.Fatal("print-only setup must not launch an agent")
		return nil
	}

	var stdout, stderr string
	var code int
	withinDirectory(t, repo, func() {
		stdout, stderr, code = captureMainOutput(t, func() int {
			return runSetup(nil)
		})
		if code != 0 {
			t.Fatalf("setup code=%d stderr=%s", code, stderr)
		}
	})
	if !strings.Contains(stdout, "Use these seed questions only when context cannot answer them") ||
		!strings.Contains(stdout, "create or update the knowledge base at Wiki") ||
		strings.Contains(stdout, "Wiki type:") {
		t.Fatalf("zero-argument setup should print the open-ended setup interview:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(repo, "Wiki")); !os.IsNotExist(err) {
		t.Fatalf("print-only setup must not create Wiki: %v", err)
	}
}

func TestParseSetupArgsKeepsExplicitTargetAsGuidedSetup(t *testing.T) {
	project, err := parseSetupArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if project.wiki != "Wiki" || project.source != "" {
		t.Fatalf("zero-argument setup=%+v, want guided setup for Wiki", project)
	}
	if project.agent {
		t.Fatalf("zero-argument setup=%+v, want print-only mode", project)
	}

	guided, err := parseSetupArgs([]string{"Knowledge"})
	if err != nil {
		t.Fatal(err)
	}
	if guided.wiki != "Knowledge" || guided.source != "" {
		t.Fatalf("explicit-target setup=%+v, want guided setup", guided)
	}

	withRules, err := parseSetupArgs([]string{"--rules", "docs"})
	if err != nil {
		t.Fatal(err)
	}
	if withRules.source != "" {
		t.Fatalf("rule-selected setup source=%q, want guided setup", withRules.source)
	}
}

func TestSelectSetupRuntimeOffersInstalledRuntimes(t *testing.T) {
	t.Setenv("OPENKNOWLEDGE_CODEX", "/test/codex")
	t.Setenv("OPENKNOWLEDGE_CLAUDE", "/test/claude")
	t.Setenv("OPENKNOWLEDGE_OPENCODE", "/test/opencode")
	originalProbe := probeCodexExecutable
	originalInput := setupRuntimeInput
	originalIsTerminal := setupRuntimeInputIsTerminal
	t.Cleanup(func() {
		probeCodexExecutable = originalProbe
		setupRuntimeInput = originalInput
		setupRuntimeInputIsTerminal = originalIsTerminal
	})
	probeCodexExecutable = func(_ context.Context, _ string) error { return nil }
	setupRuntimeInput = strings.NewReader("2\n")
	setupRuntimeInputIsTerminal = func() bool { return true }

	stdout, stderr, code := captureMainOutput(t, func() int {
		runtime, executable, err := selectSetupRuntime(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if runtime != "codex" || executable != "/test/codex" {
			t.Fatalf("selected %s at %s, want codex at /test/codex", runtime, executable)
		}
		return 0
	})
	if code != 0 || stderr != "" {
		t.Fatalf("runtime selection code=%d stderr=%q", code, stderr)
	}
	for _, expected := range []string{"1. claude", "2. codex", "3. opencode", "Select a runtime:"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("runtime selection is missing %q:\n%s", expected, stdout)
		}
	}
}

func TestSelectSetupRuntimeRequiresFlagForNonInteractiveInput(t *testing.T) {
	t.Setenv("OPENKNOWLEDGE_CODEX", "/test/codex")
	t.Setenv("OPENKNOWLEDGE_CLAUDE", "")
	t.Setenv("OPENKNOWLEDGE_OPENCODE", "")
	originalProbe := probeCodexExecutable
	originalInput := setupRuntimeInput
	originalIsTerminal := setupRuntimeInputIsTerminal
	t.Cleanup(func() {
		probeCodexExecutable = originalProbe
		setupRuntimeInput = originalInput
		setupRuntimeInputIsTerminal = originalIsTerminal
	})
	probeCodexExecutable = func(_ context.Context, candidate string) error {
		if candidate == "/test/codex" {
			return nil
		}
		return errors.New("not installed")
	}
	setupRuntimeInput = strings.NewReader("")
	setupRuntimeInputIsTerminal = func() bool { return false }

	_, _, err := selectSetupRuntime(context.Background())
	if err == nil || !strings.Contains(err.Error(), "requires --runtime when input is not interactive") {
		t.Fatalf("unexpected non-interactive selection error: %v", err)
	}
}

func TestSetupDoesNotIntegrateInvalidAgentOutput(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	wiki := filepath.Join(repo, "Wiki")
	stubCodexResolver(t, "/test/codex")
	original := runAgentProcess
	t.Cleanup(func() { runAgentProcess = original })
	runAgentProcess = func(_ context.Context, _ string, _ []string, _ string) error {
		if err := os.MkdirAll(wiki, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(wiki, "index.md"), []byte("# Wiki\n"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(wiki, "concept.md"), []byte("# Missing frontmatter\n"), 0o644)
	}

	_, _, code := captureMainOutput(t, func() int {
		return runSetup([]string{wiki, "--agent", "--runtime", "codex"})
	})
	if code == 0 {
		t.Fatal("expected invalid setup output to fail")
	}
	if _, err := os.Stat(filepath.Join(repo, ".openknowledge", "integration.toml")); !os.IsNotExist(err) {
		t.Fatalf("integration should not be installed after validation failure: %v", err)
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
	probeCodexExecutable = func(_ context.Context, _ string) error {
		t.Fatal("setup should not probe an undiscovered executable")
		return nil
	}
	runAgentProcess = func(_ context.Context, _ string, _ []string, _ string) error {
		t.Fatal("setup should not launch an unavailable runtime")
		return nil
	}

	_, stderr, code := captureMainOutput(t, func() int {
		return runSetup([]string{filepath.Join(repo, "Wiki"), "--agent", "--runtime", "codex"})
	})
	if code != 1 {
		t.Fatalf("setup code=%d stderr=%s", code, stderr)
	}
	for _, expected := range []string{
		"setup cannot start the codex runtime",
		"openknowledge agent doctor --runtime codex",
		"install or repair the runtime and rerun setup",
	} {
		if !strings.Contains(stderr, expected) {
			t.Fatalf("missing %q in setup recovery diagnostic:\n%s", expected, stderr)
		}
	}
}

func TestSetupReportsAuthenticationRecoveryAfterAgentFailure(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	stubCodexResolver(t, "/test/codex")
	original := runAgentProcess
	t.Cleanup(func() { runAgentProcess = original })
	runAgentProcess = func(_ context.Context, executable string, _ []string, _ string) error {
		if executable != "/test/codex" {
			t.Fatalf("setup executable=%q", executable)
		}
		return errors.New("authentication required")
	}

	_, stderr, code := captureMainOutput(t, func() int {
		return runSetup([]string{filepath.Join(repo, "Wiki"), "--agent", "--runtime", "codex"})
	})
	if code != 1 {
		t.Fatalf("setup code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "verify its authentication and rerun the same setup command") {
		t.Fatalf("missing authentication recovery diagnostic:\n%s", stderr)
	}
}

func TestParseSetupArgsRejectsAmbiguousModes(t *testing.T) {
	for _, args := range [][]string{
		{"Wiki", "Other"},
		{"Wiki", "--rules", "docs", "--from", "."},
		{"Wiki", "--about", "goal"},
		{"Wiki", "--runtime", "unknown"},
		{"--runtime", "codex"},
		{"--model", "gpt-test"},
	} {
		if _, err := parseSetupArgs(args); err == nil {
			t.Fatalf("expected setup args to fail: %#v", args)
		}
	}
}
