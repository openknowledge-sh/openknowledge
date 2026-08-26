package okf

import "testing"

func TestBundleValidationChecksHideInactiveExtensions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Knowledge\n")
	writeFile(t, root, "guide.md", "---\ntype: Guide\n---\n\n# Guide\n")

	result, err := ValidateWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Publish metadata", "Insights", "Rule catalog", "Typed claims", "Corpus schema"} {
		if status := statusForCheck(result, name); status != "" {
			t.Fatalf("inactive extension check %q was reported with status %q: %#v", name, status, result.Checks)
		}
	}
	if hasCheckGroup(result.Checks, "Open Knowledge extensions") {
		t.Fatalf("lightweight bundle reported an inactive extension group: %#v", result.Checks)
	}
}

func TestBundleValidationChecksShowActiveExtensions(t *testing.T) {
	tests := []struct {
		name      string
		checkName string
		write     func(t *testing.T, root string)
	}{
		{
			name:      "publish metadata",
			checkName: "Publish metadata",
			write: func(t *testing.T, root string) {
				writeFile(t, root, "index.md", "---\nokf_publish: false\n---\n\n# Knowledge\n")
			},
		},
		{
			name:      "insights",
			checkName: "Insights",
			write: func(t *testing.T, root string) {
				writeFile(t, root, "index.md", "# Knowledge\n")
				writeFile(t, root, "insights/update.md", `---
type: Open Knowledge Insight
title: Update guide
status: draft
okf_publish: false
okf_insight_id: update-guide
okf_insight_kind: explicit
generated:
  by: process:openknowledge-cli
  at: 2026-08-03T12:00:00Z
okf_insight_targets: [guide.md]
---

# Update guide
`)
			},
		},
		{
			name:      "rule catalog",
			checkName: "Rule catalog",
			write: func(t *testing.T, root string) {
				writeFile(t, root, "index.md", "# Knowledge\n")
				writeFile(t, root, "rules/security.md", `---
type: Rule
title: Security
description: Keep security guidance current.
rule_id: security
---

# Security

## Instructions

- Keep security guidance current.
`)
			},
		},
		{
			name:      "typed claims",
			checkName: "Typed claims",
			write: func(t *testing.T, root string) {
				writeFile(t, root, "index.md", "# Knowledge\n")
				writeFile(t, root, "token-evidence.txt", "Production tokens use the declared format.")
				writeFile(t, root, "auth.md", validTypedClaimDocument("okn:claim/token-format/2026-08-22", "JWT", "verified"))
			},
		},
		{
			name:      "corpus schema",
			checkName: "Corpus schema",
			write: func(t *testing.T, root string) {
				writeFile(t, root, "index.md", `---
type: Index
corpus_schema:
  version: "1"
  document_types:
    - id: Guide
      paths: [guides/**]
---

# Knowledge
`)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			test.write(t, root)

			result, err := ValidateWithVersion(root, "0.2")
			if err != nil {
				t.Fatal(err)
			}
			if status := statusForCheck(result, test.checkName); status != "pass" {
				t.Fatalf("active extension check %q has status %q: errors=%#v warnings=%#v checks=%#v", test.checkName, status, result.Errors, result.Warnings, result.Checks)
			}
		})
	}
}

func TestBundleValidationCheckStillReportsActiveExtensionFailure(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Knowledge\n")
	writeFile(t, root, "claim.md", "---\ntype: Guide\nclaims: invalid\n---\n\n# Claim\n")

	result, err := ValidateWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	if !hasIssueRule(result.Errors, ClaimValidationRule) {
		t.Fatalf("active extension validation error was lost: %#v", result.Errors)
	}
	if status := statusForCheck(result, "Typed claims"); status != "fail" {
		t.Fatalf("active invalid extension check has status %q: %#v", status, result.Checks)
	}
}
