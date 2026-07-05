package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderRulesListExplainsCommandAndRules(t *testing.T) {
	output := RenderRulesList()
	required := []string{
		"openknowledge rules prints maintenance instructions",
		"The command does not edit files",
		"openknowledge rules docs,changelog --path Wiki",
		"openknowledge rules apply docs,changelog --path Wiki --file AGENTS.md",
		"openknowledge setup --rules docs,changelog",
		"project",
		"changelog",
	}

	for _, expected := range required {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected rules list to include %q:\n%s", expected, output)
		}
	}
}

func TestRenderAgentRulesUsesSelectedRulesAndTarget(t *testing.T) {
	output, err := RenderAgentRules(AgentRulesOptions{
		Wiki:   "Wiki/",
		Target: "codex",
		Rules:  []string{"docs", "changelog"},
	})
	if err != nil {
		t.Fatal(err)
	}

	required := []string{
		"Open Knowledge wiki at `Wiki/`",
		"repository `AGENTS.md` file for Codex",
		"- Read `Wiki/index.md`",
		"- docs: Keep docs in sync",
		"Docs rules:",
		"Changelog rules:",
		"openknowledge validate \"Wiki/\"",
	}
	for _, expected := range required {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected rendered rules to include %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "Project rules:") {
		t.Fatalf("did not expect default project rules when explicit rules were selected:\n%s", output)
	}
}

func TestRulesWikiWarningsDescribeMissingAndEmptyPaths(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	warnings := RulesWikiWarnings(missing)
	if len(warnings) == 0 || !strings.Contains(warnings[0], "does not exist") || !strings.Contains(warnings[0], "Agent action: create the wiki first") {
		t.Fatalf("expected missing path warning, got %#v", warnings)
	}

	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.MkdirAll(empty, 0755); err != nil {
		t.Fatal(err)
	}
	warnings = RulesWikiWarnings(empty)
	if len(warnings) == 0 || !strings.Contains(warnings[0], "contains no Markdown") || !strings.Contains(warnings[0], "Agent action: initialize it") {
		t.Fatalf("expected empty wiki warning, got %#v", warnings)
	}
}

func TestManagedRulesBlockUpsertsIdempotently(t *testing.T) {
	first := RenderManagedRulesBlock("first")
	content := UpsertManagedRulesBlock("# Agent Rules\n", first)
	if strings.Count(content, RulesBlockStart) != 1 || !strings.Contains(content, "first") {
		t.Fatalf("expected initial managed block:\n%s", content)
	}

	second := RenderManagedRulesBlock("second")
	content = UpsertManagedRulesBlock(content, second)
	if strings.Count(content, RulesBlockStart) != 1 || !strings.Contains(content, "second") || strings.Contains(content, "first") {
		t.Fatalf("expected replacement managed block:\n%s", content)
	}
}

func TestResolveRuleSetsRejectsAliases(t *testing.T) {
	_, err := ResolveRuleSets([]string{"release-changelog"})
	if err == nil {
		t.Fatal("expected non-canonical release-changelog rule to fail")
	}
	if !strings.Contains(err.Error(), "unknown rule") {
		t.Fatalf("expected unknown rule error, got %v", err)
	}
}

func TestSetupPromptWithOptionsIncludesSelectedRules(t *testing.T) {
	prompt, err := SetupPromptWithOptions(SetupPromptOptions{Rules: []string{"docs", "changelog"}})
	if err != nil {
		t.Fatal(err)
	}
	required := []string{
		"Selected maintenance rules:",
		"- docs: Keep docs in sync",
		"- changelog: Track user-facing changes",
		"Use these as the starting point for AGENTS.md",
	}
	for _, expected := range required {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected setup prompt with rules to include %q:\n%s", expected, prompt)
		}
	}

	defaultPrompt := SetupPrompt()
	if !strings.Contains(defaultPrompt, "Available rules: project, docs, decisions, changelog, research, bugs, schemas, summary, agents.") {
		t.Fatalf("expected default setup prompt to list available rules:\n%s", defaultPrompt)
	}
}
