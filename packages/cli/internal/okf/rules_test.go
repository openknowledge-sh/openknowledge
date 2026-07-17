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
		"openknowledge prompt rules prints maintenance instructions",
		"The command does not edit files",
		"openknowledge prompt rules docs,changelog --path Wiki",
		"openknowledge prompt rules apply docs,changelog --path Wiki --file AGENTS.md",
		"openknowledge prompt setup --rules docs,changelog",
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

func TestRenderAgentRulesUsesCustomRules(t *testing.T) {
	wiki := t.TempDir()
	writeRuleTestFile(t, wiki, "rules/security.md", `---
type: Rule
title: Security
description: Keep security-sensitive changes documented.
rule_id: security
rule_review_prompt: Check recent changes for auth, secrets, permissions, or data exposure changes.
rule_review_evidence: [git diff, Wiki/security/]
---

# Security

## Instructions

- When auth, permissions, secrets, or data exposure behavior changes, update security notes.
`)

	output, err := RenderAgentRules(AgentRulesOptions{
		Wiki:  wiki,
		Rules: []string{"security"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"- security: Keep security-sensitive changes documented.",
		"Security rules:",
		"When auth, permissions, secrets, or data exposure behavior changes, update security notes.",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected rendered custom rules to include %q:\n%s", expected, output)
		}
	}

	list, err := RenderRulesListForWiki(wiki)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list, "security") || !strings.Contains(list, "Custom rules") {
		t.Fatalf("expected custom rule list:\n%s", list)
	}
}

func TestRuleCatalogConfigControlsPathsAndEnabledRules(t *testing.T) {
	wiki := t.TempDir()
	writeRuleTestFile(t, wiki, "openknowledge.toml", "[rules]\npaths = [\"policy-rules\"]\nenabled = [\"docs\", \"security\"]\n")
	writeRuleTestFile(t, wiki, "policy-rules/security.md", `---
type: Rule
title: Security
description: Keep security-sensitive changes documented.
rule_id: security
---

# Security

## Instructions

- When auth or permissions change, update security notes.
`)

	output, err := RenderAgentRules(AgentRulesOptions{Wiki: wiki})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"- docs: Keep docs in sync",
		"- security: Keep security-sensitive changes documented.",
		"Docs rules:",
		"Security rules:",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected configured default rules to include %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "Project rules:") {
		t.Fatalf("did not expect project default when rules.enabled is configured:\n%s", output)
	}

	list, err := RenderRulesListForWiki(wiki)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list, "security") {
		t.Fatalf("expected custom rule from configured path in list:\n%s", list)
	}
}

func TestParseRuleCatalogConfigValidatesPathsAndEnabled(t *testing.T) {
	config, err := ParseRuleCatalogConfig("[rules]\npaths = [\"rules\", \"policy-rules\"]\nenabled = \"docs\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(config.Paths, ",") != "rules,policy-rules" || strings.Join(config.Enabled, ",") != "docs" {
		t.Fatalf("unexpected rules config: %#v", config)
	}
	if !config.PathsConfigured || !config.EnabledConfigured {
		t.Fatalf("expected configured flags: %#v", config)
	}

	if _, err := ParseRuleCatalogConfig("[rules]\npaths = [\"../outside\"]\n"); err == nil {
		t.Fatal("expected escaping rules path to fail")
	}
	if _, err := ParseRuleCatalogConfig("[rules]\nunknown = \"value\"\n"); err == nil {
		t.Fatal("expected unknown [rules] key to fail")
	}
}

func TestValidateRuleCatalogReportsInvalidCustomRules(t *testing.T) {
	wiki := t.TempDir()
	writeRuleTestFile(t, wiki, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Wiki\n")
	writeRuleTestFile(t, wiki, "rules/docs.md", `---
type: Rule
description: Duplicate built-in docs rule.
rule_id: docs
---

# Docs

## Instructions

- Keep docs updated.
`)

	result, err := Validate(wiki)
	if err != nil {
		t.Fatal(err)
	}
	if countRule(result.Errors, "rule-catalog") != 1 {
		t.Fatalf("expected one rule-catalog error, got %#v", result.Errors)
	}
}

func TestValidateRuleCatalogReportsInvalidConfiguredEnabledRule(t *testing.T) {
	wiki := t.TempDir()
	writeRuleTestFile(t, wiki, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Wiki\n")
	writeRuleTestFile(t, wiki, "openknowledge.toml", "[rules]\nenabled = [\"missing-rule\"]\n")

	result, err := Validate(wiki)
	if err != nil {
		t.Fatal(err)
	}
	if countRule(result.Errors, "rule-catalog") != 1 {
		t.Fatalf("expected one rule-catalog error, got %#v", result.Errors)
	}
}

func TestValidateRuleCatalogReportsMissingConfiguredPath(t *testing.T) {
	wiki := t.TempDir()
	writeRuleTestFile(t, wiki, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Wiki\n")
	writeRuleTestFile(t, wiki, "openknowledge.toml", "[rules]\npaths = [\"missing-rules\"]\n")

	result, err := Validate(wiki)
	if err != nil {
		t.Fatal(err)
	}
	if countRule(result.Errors, "rule-catalog") != 1 {
		t.Fatalf("expected one rule-catalog error, got %#v", result.Errors)
	}
}

func TestCustomRuleSetsRejectsMalformedRuleDocuments(t *testing.T) {
	wiki := t.TempDir()
	writeRuleTestFile(t, wiki, "rules/broken.md", "---\ntype: Rule\n")

	_, err := CustomRuleSets(wiki)
	if err == nil {
		t.Fatal("expected malformed custom rule document to fail")
	}
	if !strings.Contains(err.Error(), "custom rule catalog") || !strings.Contains(err.Error(), "frontmatter") {
		t.Fatalf("expected rule-catalog frontmatter error, got %v", err)
	}
}

func TestRenderRuleReviewPromptIncludesSelectedRuleEvidence(t *testing.T) {
	wiki := t.TempDir()
	writeRuleTestFile(t, wiki, "rules/security.md", `---
type: Rule
title: Security
description: Keep security-sensitive changes documented.
rule_id: security
rule_review_prompt: Check recent changes for auth, secrets, permissions, or data exposure changes.
rule_review_evidence: [git diff, Wiki/security/]
---

# Security

## Instructions

- When auth, permissions, secrets, or data exposure behavior changes, update security notes.
`)

	output, err := RenderRuleReviewPrompt(RuleReviewOptions{
		Wiki:  wiki,
		Rules: []string{"security"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"Open Knowledge Rule Review",
		"advisory AI review",
		"openknowledge validate",
		"security: Keep security-sensitive changes documented.",
		"Check recent changes for auth, secrets, permissions, or data exposure changes.",
		"git diff",
		"Wiki/security/",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected review prompt to include %q:\n%s", expected, output)
		}
	}
}

func TestRulesWikiWarningsDescribeMissingAndEmptyPaths(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	warnings := RulesWikiWarnings(missing)
	if len(warnings) == 0 || !strings.Contains(warnings[0], "does not exist") || !strings.Contains(warnings[0], "Agent action: create the wiki first") || !strings.Contains(warnings[0], "openknowledge scaffold") {
		t.Fatalf("expected missing path warning, got %#v", warnings)
	}

	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.MkdirAll(empty, 0755); err != nil {
		t.Fatal(err)
	}
	warnings = RulesWikiWarnings(empty)
	if len(warnings) == 0 || !strings.Contains(warnings[0], "contains no Markdown") || !strings.Contains(warnings[0], "Agent action: initialize it") || !strings.Contains(warnings[0], "openknowledge scaffold") {
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

func writeRuleTestFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
