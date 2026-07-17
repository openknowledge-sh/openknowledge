package main

import (
	"context"
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
		return runSetup([]string{wiki, "--rules", "docs"})
	})
	if code != 0 {
		t.Fatalf("setup code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(prompt, "This setup guide is meant to be executed") || !strings.Contains(prompt, "Knowledge") || !strings.Contains(prompt, "Selected maintenance rules") {
		t.Fatalf("unexpected setup prompt:\n%s", prompt)
	}
	for _, path := range []string{".openknowledge/integration.toml", ".agents/skills/openknowledge/SKILL.md", ".codex/hooks.json"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing integration file %s: %v", path, err)
		}
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
		return runSetup([]string{wiki, "--from", ".", "--type", "custom", "--about", "Explain releases"})
	})
	if code != 0 {
		t.Fatalf("setup --from code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(prompt, "Source: `.`") || !strings.Contains(prompt, "Output wiki path: `Wiki`") || !strings.Contains(prompt, "Explain releases") {
		t.Fatalf("unexpected source workflow prompt:\n%s", prompt)
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
		return runSetup([]string{wiki})
	})
	if code == 0 {
		t.Fatal("expected invalid setup output to fail")
	}
	if _, err := os.Stat(filepath.Join(repo, ".openknowledge", "integration.toml")); !os.IsNotExist(err) {
		t.Fatalf("integration should not be installed after validation failure: %v", err)
	}
}

func TestParseSetupArgsRejectsAmbiguousModes(t *testing.T) {
	for _, args := range [][]string{
		{"Wiki", "Other"},
		{"Wiki", "--rules", "docs", "--from", "."},
		{"Wiki", "--about", "goal"},
		{"Wiki", "--runtime", "unknown"},
	} {
		if _, err := parseSetupArgs(args); err == nil {
			t.Fatalf("expected setup args to fail: %#v", args)
		}
	}
}
