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
		"openknowledge setup --prompt --rules docs,changelog",
		"project",
		"writing",
		"iso-plain-language",
		"changelog",
	}

	for _, expected := range required {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected rules list to include %q:\n%s", expected, output)
		}
	}
}

func TestDefaultRulesIncludeProjectAndWriting(t *testing.T) {
	ruleSets, err := ResolveRuleSets(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ruleSets) != 2 || ruleSets[0].ID != "project" || ruleSets[1].ID != "writing" {
		t.Fatalf("unexpected default rules: %#v", ruleSets)
	}

	writing := ruleSets[1]
	if len(writing.Rules) != 10 || !strings.Contains(writing.ReviewPrompt, "task-focused") {
		t.Fatalf("unexpected writing rule: %#v", writing)
	}

	iso, err := ResolveRuleSets([]string{"iso-plain-language"})
	if err != nil {
		t.Fatal(err)
	}
	if len(iso) != 1 || len(iso[0].Rules) != 6 || !strings.Contains(iso[0].ReviewPrompt, "Do not claim ISO certification") {
		t.Fatalf("unexpected ISO plain language rule: %#v", iso)
	}
}

func TestRenderAgentRulesUsesDefaultProjectAndWritingRules(t *testing.T) {
	wiki := t.TempDir()
	writeRuleTestFile(t, wiki, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Wiki\n")

	output, err := RenderAgentRules(AgentRulesOptions{Wiki: wiki})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Project rules:", "Writing rules:"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected default output to include %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "ISO 24495-1 Plain-Language Principles rules:") {
		t.Fatalf("did not expect optional ISO rule in default output:\n%s", output)
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
	writeRuleTestFile(t, wiki, ValidationConfigFile, "[rules]\npaths = [\"policy-rules\"]\nenabled = [\"docs\", \"security\"]\n")
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
	writeRuleTestFile(t, wiki, ValidationConfigFile, "[rules]\nenabled = [\"missing-rule\"]\n")

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
	writeRuleTestFile(t, wiki, ValidationConfigFile, "[rules]\npaths = [\"missing-rules\"]\n")

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
		"Use these as the starting point for the knowledge base, AGENTS.md",
		`enabled = ["docs", "changelog"]`,
		`okn scaffold --name "<knowledge base name>" --rules "docs,changelog" "<folder path>"`,
		"### docs",
		"Keep docs focused on shipped behavior",
	}
	for _, expected := range required {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected setup prompt with rules to include %q:\n%s", expected, prompt)
		}
	}

	defaultPrompt := SetupPrompt()
	if !strings.Contains(defaultPrompt, "Default rules: project and writing. Optional rules: iso-plain-language, docs, decisions, changelog, research, bugs, schemas, summary, and agents.") {
		t.Fatalf("expected default setup prompt to list available rules:\n%s", defaultPrompt)
	}
	for _, expected := range []string{
		`enabled = ["project", "writing"]`,
		`okn scaffold --name "<knowledge base name>" --rules "project,writing" "<folder path>"`,
		"### writing",
		"Start with the reader's task or required answer.",
	} {
		if !strings.Contains(defaultPrompt, expected) {
			t.Fatalf("expected default setup prompt to include %q:\n%s", expected, defaultPrompt)
		}
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
