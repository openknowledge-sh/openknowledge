package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateConformanceBySpecVersion(t *testing.T) {
	for _, version := range SupportedSpecVersions() {
		t.Run(version+"/valid_bundle", func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "index.md", "---\nokf_version: \""+version+"\"\n---\n\n# Bundle\n")
			writeFile(t, root, "log.md", "# Log\n\n## 2026-06-15\n\n* **Creation**: Created bundle.\n")
			writeFile(t, root, "concepts/table.md", "---\ntype: BigQuery Table\ntitle: Orders\n---\n\n# Schema\n")

			result, err := ValidateWithVersion(root, version)
			if err != nil {
				t.Fatal(err)
			}
			if result.SpecVersion != version {
				t.Fatalf("expected spec version %s, got %s", version, result.SpecVersion)
			}
			if len(result.Errors) != 0 {
				t.Fatalf("expected no errors, got %#v", result.Errors)
			}
			if statusForCheck(result, "Bundle scan") != "pass" {
				t.Fatalf("expected bundle scan to pass, got %#v", result.Checks)
			}
		})

		t.Run(version+"/scanner_includes_uppercase_markdown", func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "SETUP.MD", "---\ntype: Setup\ntitle: Setup\n---\n")

			result, err := ValidateWithVersion(root, version)
			if err != nil {
				t.Fatal(err)
			}
			if result.Concepts != 1 || result.Files != 1 {
				t.Fatalf("expected uppercase Markdown file to be scanned, got %#v", result)
			}
		})

		t.Run(version+"/scanner_includes_markdown_extension", func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "guide.markdown", "---\ntype: Guide\ntitle: Guide\n---\n\n# Guide\n")

			result, err := ValidateWithVersion(root, version)
			if err != nil {
				t.Fatal(err)
			}
			if result.Concepts != 1 || result.Files != 1 {
				t.Fatalf("expected .markdown file to be scanned, got %#v", result)
			}
		})

		t.Run(version+"/missing_concept_type_fails", func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "concept.md", "---\ntitle: Missing Type\n---\n")

			result, err := ValidateWithVersion(root, version)
			if err != nil {
				t.Fatal(err)
			}
			if statusForCheck(result, "Concept documents") != "fail" {
				t.Fatalf("expected concept documents check to fail, got %#v", result.Checks)
			}
		})

		t.Run(version+"/reserved_files_fail", func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "docs/index.md", "---\ntype: Index\n---\n# Docs\n")
			writeFile(t, root, "log.md", "# Log\n\n## June 15 2026\n\n* Bad date.\n")

			result, err := ValidateWithVersion(root, version)
			if err != nil {
				t.Fatal(err)
			}
			if statusForCheck(result, "Reserved files") != "fail" || statusForCheck(result, "Log dates") != "fail" {
				t.Fatalf("expected reserved file and log date checks to fail, got %#v", result.Checks)
			}
		})

		t.Run(version+"/generated_scaffold_validates", func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "knowledge")

			created, err := NewProject(NewProjectOptions{Name: "Knowledge", Path: target, SpecVersion: version})
			if err != nil {
				t.Fatal(err)
			}
			if created.SpecVersion != version {
				t.Fatalf("expected scaffold result to select %s, got %#v", version, created)
			}
			index, err := os.ReadFile(filepath.Join(target, "index.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(index), `okf_version: "`+version+`"`) {
				t.Fatalf("expected scaffold to declare OKF %s:\n%s", version, index)
			}
			spec, err := os.ReadFile(filepath.Join(target, "SPEC.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(spec), "Version "+version) {
				t.Fatalf("expected scaffold to embed OKF %s:\n%s", version, spec)
			}
			agentRules, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
			if err != nil {
				t.Fatal(err)
			}
			if version == "0.1" {
				if !strings.Contains(string(agentRules), "timestamp:") || strings.Contains(string(agentRules), "generated:") {
					t.Fatalf("expected OKF 0.1 scaffold metadata:\n%s", agentRules)
				}
			} else if !strings.Contains(string(agentRules), "generated:") {
				t.Fatalf("expected OKF 0.2 scaffold metadata:\n%s", agentRules)
			}
			result, err := ValidateWithVersion(target, version)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Errors) != 0 {
				t.Fatalf("expected scaffold to validate against %s, got %#v", version, result.Errors)
			}
		})
	}
}

func TestNewProjectDefaultsToLatestSpecAndRejectsUnsupportedVersion(t *testing.T) {
	target := filepath.Join(t.TempDir(), "latest")
	created, err := NewProject(NewProjectOptions{Name: "Latest", Path: target})
	if err != nil {
		t.Fatal(err)
	}
	if created.SpecVersion != LatestSpecVersion {
		t.Fatalf("expected default scaffold to select latest %s, got %#v", LatestSpecVersion, created)
	}

	unsupportedTarget := filepath.Join(t.TempDir(), "unsupported")
	if _, err := NewProject(NewProjectOptions{Name: "Unsupported", Path: unsupportedTarget, SpecVersion: "9.9"}); err == nil {
		t.Fatal("expected unsupported scaffold spec to fail")
	}
	if _, err := os.Stat(unsupportedTarget); !os.IsNotExist(err) {
		t.Fatalf("unsupported spec must fail before creating the target, got %v", err)
	}
}

func TestValidateRejectsUnsupportedSpecVersion(t *testing.T) {
	_, err := ValidateWithVersion(t.TempDir(), "9.9")
	if err == nil {
		t.Fatal("expected unsupported spec version error")
	}
}

func TestValidationProfilesBindEveryRuleToASpecVersion(t *testing.T) {
	expected := map[string]map[string]validationRuleDefinition{
		"0.1": {
			"bundle-read":         {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"claim-profile":       {DefaultSeverity: ValidationSeverityError},
			"corpus-schema":       {DefaultSeverity: ValidationSeverityError},
			"concept-frontmatter": {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"concept-type":        {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"frontmatter":         {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"frontmatter-format":  {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"index-frontmatter":   {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"insight-contract":    {DefaultSeverity: ValidationSeverityError},
			"link-target":         {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"log-date":            {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"log-frontmatter":     {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"markdown-syntax":     {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"okf-version":         {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"publish-metadata":    {DefaultSeverity: ValidationSeverityError},
			"rule-catalog":        {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"utf-8":               {DefaultSeverity: ValidationSeverityError, Overrideable: true},
		},
		"0.2": {
			"bundle-read":         {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"claim-profile":       {DefaultSeverity: ValidationSeverityError},
			"corpus-schema":       {DefaultSeverity: ValidationSeverityError},
			"concept-frontmatter": {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"concept-type":        {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"frontmatter":         {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"frontmatter-format":  {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"index-frontmatter":   {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"insight-contract":    {DefaultSeverity: ValidationSeverityError},
			"link-target":         {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"log-date":            {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"log-frontmatter":     {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"markdown-syntax":     {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"okf-0.2-metadata":    {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"okf-version":         {DefaultSeverity: ValidationSeverityWarning, Overrideable: true},
			"publish-metadata":    {DefaultSeverity: ValidationSeverityError},
			"rule-catalog":        {DefaultSeverity: ValidationSeverityError, Overrideable: true},
			"utf-8":               {DefaultSeverity: ValidationSeverityError, Overrideable: true},
		},
	}

	for version, rules := range expected {
		profile, err := validationProfileForVersion(version)
		if err != nil {
			t.Fatal(err)
		}
		if len(profile.Rules) != len(rules) {
			t.Fatalf("OKF %s profile has %d rules; expected %d: %#v", version, len(profile.Rules), len(rules), profile.Rules)
		}
		for rule, definition := range rules {
			if actual, ok := profile.Rules[rule]; !ok || actual != definition {
				t.Fatalf("OKF %s rule %q: got %#v, expected %#v", version, rule, actual, definition)
			}
		}
	}
}

func TestValidationRuleDiscoveryAndOverridesAreVersionBound(t *testing.T) {
	v01Rules, err := KnownValidationRulesForVersion("0.1")
	if err != nil {
		t.Fatal(err)
	}
	v02Rules, err := KnownValidationRulesForVersion("0.2")
	if err != nil {
		t.Fatal(err)
	}
	if containsString(v01Rules, "okf-0.2-metadata") {
		t.Fatalf("0.1 must not advertise the 0.2-only rule: %v", v01Rules)
	}
	if !containsString(v02Rules, "okf-0.2-metadata") {
		t.Fatalf("0.2 must advertise its metadata rule: %v", v02Rules)
	}
	if defined, overrideable := validationRuleAcrossProfiles("okf-0.2-metadata"); !defined || !overrideable {
		t.Fatalf("expected 0.2 metadata to be a configurable rule across supported profiles")
	}
	if defined, _ := validationRuleAcrossProfiles("not-a-rule"); defined {
		t.Fatal("globally unknown rules must not be defined")
	}
	for _, version := range SupportedSpecVersions() {
		if !IsKnownValidationRuleForVersion(version, "publish-metadata") || IsValidationRuleOverrideableForVersion(version, "publish-metadata") {
			t.Fatalf("OKF %s must define publish-metadata as fixed severity", version)
		}
	}
	if _, _, err := ParseValidationRuleOverrideForVersion("0.1", "okf-0.2-metadata=warn"); err == nil || !strings.Contains(err.Error(), "not defined for OKF 0.1") {
		t.Fatalf("expected a version-bound override error, got %v", err)
	}
	if _, _, err := ParseValidationRuleOverrideForVersion("0.2", "publish-metadata=off"); err == nil || !strings.Contains(err.Error(), "fixed severity for OKF 0.2") {
		t.Fatalf("expected a fixed-severity error, got %v", err)
	}
}

func TestValidationConfigActivatesOnlyRulesForSelectedVersion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Bundle\n")
	options := ValidationOptions{Rules: map[string]string{"okf-0.2-metadata": "error"}}

	legacy, err := ValidateWithVersionAndOptions(root, "0.1", options)
	if err != nil {
		t.Fatalf("expected 0.1 to ignore a known inactive 0.2 rule: %v", err)
	}
	if _, active := legacy.Policy.Overrides["okf-0.2-metadata"]; active {
		t.Fatalf("0.1 must not activate the 0.2-only override: %#v", legacy.Policy)
	}
	current, err := ValidateWithVersionAndOptions(root, "0.2", options)
	if err != nil {
		t.Fatalf("expected 0.2 to accept its metadata rule: %v", err)
	}
	if current.Policy.Overrides["okf-0.2-metadata"] != ValidationSeverityError {
		t.Fatalf("0.2 must activate its metadata override: %#v", current.Policy)
	}
	if _, err := ValidateWithVersionAndOptions(root, "0.2", ValidationOptions{
		Rules: map[string]string{"not-a-rule": "error"},
	}); err == nil || !strings.Contains(err.Error(), "unknown validation rule") {
		t.Fatalf("expected a globally unknown rule to fail, got %v", err)
	}
}

func TestValidationRejectsAnEmittedRuleOutsideTheSelectedProfile(t *testing.T) {
	result := Result{
		SpecVersion: "0.2",
		Errors:      []Issue{{Rule: "unprofiled-check", Message: "programmer error"}},
	}
	if err := applyValidationOptions(&result, ValidationOptions{}); err == nil || !strings.Contains(err.Error(), "not defined for OKF 0.2") {
		t.Fatalf("expected an unprofiled emitted rule to fail closed, got %v", err)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func TestOKFV02MetadataValidationAndVerifiedNormalization(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.2\"\n---\n\n# Bundle\n")
	writeFile(t, root, "revenue.md", `---
type: Attested Computation
title: Revenue
runtime: bigquery
parameters:
  - { name: year, type: integer, required: true }
generated: { by: reference_agent/gemini-2.5-pro, at: 2026-06-20T22:53:05Z }
verified: { by: human:reviewer, at: 2026-06-25T09:00:00Z }
status: stable
stale_after: 2026-12-31T00:00:00Z
sources:
  - id: revenue-policy
    resource: https://example.com/revenue-policy
    author: team:finance
    usage_count: 50
    last_modified: 2026-06-01T00:00:00Z
usage_window: { from: 2026-06-01T00:00:00Z, to: 2026-06-30T00:00:00Z }
---

# Computation

    SELECT SUM(amount) FROM revenue WHERE fiscal_year = @year

This follows policy.[^revenue-policy]

[^revenue-policy]: Revenue policy
`)

	result, err := ValidateWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 0 || countRule(result.Warnings, "okf-0.2-metadata") != 0 {
		t.Fatalf("expected valid 0.2 metadata, got errors=%#v warnings=%#v", result.Errors, result.Warnings)
	}
	if statusForCheck(result, "OKF 0.2 metadata") != "pass" {
		t.Fatalf("expected 0.2 metadata check to pass, got %#v", result.Checks)
	}

	ast, err := ParseASTWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	verified, ok := ast.Documents[1].Frontmatter.Data["verified"].([]any)
	if !ok || len(verified) != 1 {
		t.Fatalf("expected bare verified mapping to normalize to one event, got %#v", ast.Documents[1].Frontmatter.Data["verified"])
	}

	legacy, err := ParseASTWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := legacy.Documents[1].Frontmatter.Data["verified"].(map[string]any); !ok {
		t.Fatalf("expected 0.1 parser to preserve the authored mapping, got %#v", legacy.Documents[1].Frontmatter.Data["verified"])
	}
}

func TestOKFV02MetadataGuidanceIsNonBlocking(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "broken.md", `---
type: Attested Computation
generated: { at: yesterday }
verified: [{ by: human:, at: soon }]
status: retired
stale_after: tomorrow
sources:
  - id: duplicate
  - id: duplicate
    resource: https://example.com
parameters: [{ name: value, type: number, required: yes }]
executor: { receipt: result }
---

Unknown source.[^missing]
`)

	result, err := ValidateWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("0.2 optional-family guidance must not reject the bundle: %#v", result.Errors)
	}
	if countRule(result.Warnings, "okf-0.2-metadata") < 10 {
		t.Fatalf("expected focused 0.2 metadata warnings, got %#v", result.Warnings)
	}
	if statusForCheck(result, "OKF 0.2 metadata") != "warn" {
		t.Fatalf("expected 0.2 metadata check to warn, got %#v", result.Checks)
	}
	for _, warning := range result.Warnings {
		if warning.Rule == "okf-0.2-metadata" && strings.TrimSpace(warning.Message) == "" {
			t.Fatalf("expected actionable warning: %#v", warning)
		}
	}
}
